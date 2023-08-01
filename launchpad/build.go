package launchpad

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/versions"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/moby/buildkit/frontend/dockerfile/dockerignore"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/sshforward/sshprovider"
	"github.com/moby/buildkit/util/progress/progressui"
	"github.com/moby/term"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"github.com/spf13/afero"
	"go.jetpack.io/launchpad/goutil/errorutil"
	"go.jetpack.io/launchpad/launchpad/authprovider"
	"go.jetpack.io/launchpad/padcli/hook"
	"go.jetpack.io/launchpad/padcli/jetconfig"
	"go.jetpack.io/launchpad/padcli/provider"
	"go.jetpack.io/launchpad/pkg/docker"
	"go.jetpack.io/launchpad/pkg/jetlog"
	"go.jetpack.io/launchpad/pkg/kubevalidate"

	controlapi "github.com/moby/buildkit/api/services/control"
	buildkitClient "github.com/moby/buildkit/client"
)

const (
	dockerProjectIdLabel = "jetpack.io/project-id"
)

type BuildOptions struct {
	// fields in abc order

	AppName string

	BuildArgs map[string]string

	LifecycleHook hook.LifecycleHook

	// Pre-built local image to use
	LocalImage string

	// ProjectDir is the absolute path to the module
	ProjectDir string

	ProjectId string

	// Platform informs docker which architecture to build for.
	// Examples: linux/arm64 (for M1 macs) linux/amd64 (for intel macs)
	Platform string

	Services map[string]jetconfig.Builder

	RemoteCache bool

	RepoConfig        provider.RepoConfig // required for remote cache feature
	ImageRepoForCache string

	TagPrefix string
}

func (b *BuildOptions) GetBuilders() map[string]jetconfig.Builder {
	return lo.PickBy(
		b.Services,
		func(name string, s jetconfig.Builder) bool {
			return s.GetBuildCommand() != ""
		},
	)
}

func (b *BuildOptions) GetRepoHost() string {
	if b == nil || b.RepoConfig == nil {
		return ""
	}
	return strings.Split(b.RepoConfig.GetImageRepoPrefix(), "/")[0]
}

type BuildPlan struct {
	// keep fields in abc order.

	dockerfilePath string

	// Best explained via example. If the full URL of the image is:
	// us-central1-docker.pkg.dev/jetpack-dev/jetpack-internal-demo/py-hello-world:34fd45
	//
	// then:
	// registry.uri is us-central1-docker.pkg.dev
	// imageName is py-hello-world
	// imageRepo is jetpack-dev/jetpack-internal-demo/py-hello-world
	// imageTag is 34fd45
	image       *LocalImage
	imageLabels map[string]string

	// projectDir is the absolute path to the directory of the project
	projectDir string

	buildOpts *BuildOptions
}

func (p *BuildPlan) requiresDockerfile() bool {
	return planShouldUseDockerfile(p.buildOpts)
}

type BuildOutput struct {
	Duration time.Duration
	Image    *LocalImage
}

func (o *BuildOutput) DidBuildUsingDockerfile() bool {
	return o != nil && o.Image != nil && *o.Image != ""
}

func (o *BuildOutput) SetDuration(d time.Duration) {
	if o != nil {
		o.Duration = d
	}
}

// build is the internal implementation of Build. See docs there.
func build(
	ctx context.Context,
	fs afero.Fs,
	opts *BuildOptions,
) (*BuildOutput, error) {

	plan, err := makeBuildPlan(ctx, opts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to make build plan")
	}

	err = validateBuildPlan(plan, fs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to validate build plan")
	}

	err = executeBuildPlan(ctx, plan)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute build plan")
	}

	return &BuildOutput{Image: plan.image}, nil
}

func makeBuildPlan(
	ctx context.Context,
	opts *BuildOptions,
) (*BuildPlan, error) {

	useDockerfile := planShouldUseDockerfile(opts)

	dockerfilePath := ""
	imageName := ""
	imageTag := ""
	if useDockerfile {
		var err error
		dockerfilePath, err = getDockerfilePath(opts.ProjectDir)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		if dockerfilePath != "" {
			imageName, imageTag, err = getImageNameAndTag(ctx, opts)
			if err != nil {
				return nil, errors.WithStack(err)
			}
		}
	} else if opts.LocalImage != "" {
		imageInfo := strings.Split(opts.LocalImage, ":")
		if len(imageInfo) > 0 {
			imageName = imageInfo[0]
			if len(imageInfo) == 2 {
				imageTag = imageInfo[1]
			}
		}
	}

	plan := &BuildPlan{
		dockerfilePath: dockerfilePath,
		image:          newLocalImageWithTag(imageName, imageTag),
		projectDir:     opts.ProjectDir,
		imageLabels:    map[string]string{dockerProjectIdLabel: opts.ProjectId},
		buildOpts:      opts,
	}
	return plan, nil
}

func planShouldUseDockerfile(opts *BuildOptions) bool {
	if len(opts.Services) == 0 {
		return true
	}

	hasServiceWithEmptyImage := lo.SomeBy(
		lo.Values(opts.Services),
		func(s jetconfig.Builder) bool {
			return s.GetImage() == ""
		},
	)

	// If any service doesn't have an image specified,
	// then we need to build a docker image for it to use.
	return hasServiceWithEmptyImage && opts.LocalImage == ""
}

// getDockerfilePath returns the location of the docker file in the module directory.
// A return value of an empty string and nil error means that no Dockerfile was found.
func getDockerfilePath(modulePath string) (string, error) {
	path := filepath.Join(modulePath, "Dockerfile")
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", errors.WithStack(err)
	}

	return path, nil
}

func validateBuildPlan(plan *BuildPlan, fs afero.Fs) error {
	// verify that the projectDir is a directory
	isDir, err := afero.DirExists(fs, plan.projectDir)
	if err != nil {
		return errors.WithStack(err)
	}
	if !isDir {
		return errors.Errorf(
			"Expect module-path to be a directory, but %s is not",
			plan.projectDir,
		)
	}

	if plan.dockerfilePath == "" && plan.requiresDockerfile() {
		return errorutil.NewUserError(
			"Dockerfile missing.\n" +
				"- Please add a Dockerfile manually under your app directory.\n" +
				"- You can find an example Dockerfile at " +
				"https://github.com/jetpack-io/project-templates/blob/main/api/Dockerfile.\n" +
				"- Alternatively, to use a pre-existing image, you can add an Image: field to your service in" +
				" launchpad.yaml or jetconfig.yaml.",
		)
	}

	return nil
}

func executeBuildPlan(ctx context.Context, plan *BuildPlan) error {
	for name, b := range plan.buildOpts.GetBuilders() {
		jetlog.Logger(ctx).Printf(
			"Working dir: %s\nBuilding service %s using \"%s\"\n",
			b.GetPath(),
			name,
			b.GetBuildCommand(),
		)
		cmd := exec.CommandContext(
			ctx,
			"/bin/sh",
			"-c",
			b.GetBuildCommand(),
		)
		cmd.Dir = b.GetPath()
		out, err := cmd.CombinedOutput()
		// TODO(landau): We could return output here.
		jetlog.Logger(ctx).Print(string(out))
		if err != nil {
			return errors.Wrapf(err, "failed to execute build command for service %s", name)
		}
	}

	if plan.dockerfilePath != "" {
		if err := executePlanUsingDocker(ctx, plan); err != nil {
			return errors.Wrap(err, "failed to execute build plan using docker")
		}
	}

	return nil
}

func executePlanUsingDocker(ctx context.Context, plan *BuildPlan) (err error) {
	jetlog.Logger(ctx).IndentedPrintf("Building Docker image with Dockerfile at: %s\n", plan.dockerfilePath)

	// Pulling out in case we want to allow customizing this var in the future.
	dockerfile := "Dockerfile"

	imageBuildOptions := dockertypes.ImageBuildOptions{
		BuildArgs: lo.MapValues(plan.buildOpts.BuildArgs, func(val, _ string) *string {
			return &val
		}),
		Dockerfile:     dockerfile,
		Platform:       plan.buildOpts.Platform,
		SuppressOutput: false,
		Tags:           []string{plan.image.String()},
		Labels:         plan.imageLabels,
	}

	if useCli, _ := strconv.ParseBool(os.Getenv("LAUNCHPAD_USE_DOCKER_CLI")); useCli {
		return docker.Build(filepath.Dir(plan.dockerfilePath), imageBuildOptions)
	}

	cli, err := dockerclient.NewClientWithOpts(
		dockerclient.FromEnv,
		dockerclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return errors.WithStack(err)
	}

	version, err := getDockerBuilderVersion(ctx, plan.projectDir)
	if err != nil {
		return errors.Wrap(err, "failed to detect docker builder version")
	}
	imageBuildOptions.Version = version

	buildCtx, err := dockerBuildContext(plan, dockerfile)
	if err != nil {
		return errors.Wrap(err, "tar docker build context")
	}
	defer buildCtx.Close()

	// TODO: docker.Build does not currently implement remote cache
	if plan.buildOpts.RemoteCache {
		imageBuildOptions.BuildArgs["BUILDKIT_INLINE_CACHE"] = lo.ToPtr("1")
		c, err := decodeCredentials(plan.buildOpts.RepoConfig)
		if err != nil {
			return errors.WithStack(err)
		}
		imageBuildOptions.AuthConfigs = map[string]dockertypes.AuthConfig{
			plan.buildOpts.GetRepoHost(): {
				Username: c.Username,
				Password: c.Password,
			},
		}
		if plan.buildOpts.ImageRepoForCache != "" {
			imageBuildOptions.CacheFrom = []string{
				plan.buildOpts.ImageRepoForCache + ":latest",
			}
		}
	}

	if err = initBuildkitSession(ctx, cli, &imageBuildOptions, plan); err != nil {
		return errors.WithStack(err)
	}

	// Sometimes Docker fails with a "no active session" error.
	// There isn't a reliable fix for this other than retry. This
	// retry mechanism reduces the chance of that error persisting.
	b := backoff.NewExponentialBackOff()
	// 3 minutes seems long, we can shorten this if we observe retry is taking too long
	b.MaxElapsedTime = 3 * time.Minute
	resp := dockertypes.ImageBuildResponse{}
	imageBuildRetrier := func() error {
		resp, err = cli.ImageBuild(ctx, buildCtx, imageBuildOptions)
		if err != nil {
			if strings.Contains(err.Error(), "no active session") {
				jetlog.Logger(ctx).Print(
					"\nERROR: No active Buildkit session found. Retrying...\n",
				)
			}
			return err
		} else {
			// if error is not "no active session" we don't need to retry
			return backoff.Permanent(err)
		}
	}
	err = backoff.Retry(imageBuildRetrier, b)
	if err != nil {
		return errors.Wrapf(
			err,
			"failed docker image build for %s",
			plan.image.String(),
		)
	}

	defer resp.Body.Close()

	if version == dockertypes.BuilderBuildKit {
		customWriter := dockerWriter{}
		err = writeDockerOutput(ctx, customWriter, resp)
		return errors.Wrap(err, "failed to print docker image build output")
	}

	// thank you: https://stackoverflow.com/a/58742917
	termFd, isTerm := term.GetFdInfo(os.Stderr)
	customWriter := dockerWriter{}
	err = jsonmessage.DisplayJSONMessagesStream(resp.Body, customWriter, termFd, isTerm, nil /*auxCallback */)
	return errors.Wrap(err, "failed to print image-build output")
}

// writeDockerOutput will convert the buildkit graph structures to be printable.
//
// Specifically:
// - it converts the `resp types.ImageBuildResponse` argument into JSONMessage structs
// - JSONMessage.Aux has protobuf messages of the buildkit graph
// - These protobuf messages are converted to buildkit's golang structs, which are placed into SolveStatus struct.
// - SolveStatus structs are consumed by moby's progressui library to be printed
//
// inspired by:
// https://github.com/docker/docker-ce/blob/master/components/cli/cli/command/image/build_buildkit.go
func writeDockerOutput(ctx context.Context, jetpackPrinter io.Writer, resp dockertypes.ImageBuildResponse) error {

	// This WaitGroup is needed to ensure that all the solveStatus values below are printed
	// prior to exiting. Without this, the next launchpad-step's output (publish) may begin printing.
	var wg sync.WaitGroup
	defer wg.Wait()

	// This channel is used to send SolveStatus from auxCallback to progressui.DisplaySolveStatus
	solveStatus := make(chan *buildkitClient.SolveStatus)
	defer close(solveStatus)

	// The caller must wait for this goroutine to print all SolveStatuses
	wg.Add(1)
	go func() {
		defer wg.Done()

		// A console can be used to print docker build output, and then "rewrite it" so that
		// it becomes compact if the build steps were successful. This is similar to what
		// docker CLI does for `docker image build`.
		//
		// However, due to how `jetpackPrinter` rewrites the output (via indentation)
		// the progressui output gets messed up. Hence, this is commented out for now.
		//
		// For now, we can pass "nil" as console to progressui.DisplaySolveStatus.
		//
		// import "github.com/containerd/console"
		//cons, err := console.ConsoleFromFile(jetpackPrinter)
		//if err != nil {
		//	return errors.Wrap(err, "failed to set console")
		//}

		_, err := progressui.DisplaySolveStatus(ctx, "", nil /*console*/, jetpackPrinter, solveStatus)
		if err != nil {
			// Note, we are okay printing and continuing due to this error.
			// The progressui is for printing docker build info, but we shouldn't
			// block a deploy if there is some error due to it.
			fmt.Printf("ERROR: from DisplaySolveStatus. %v\n", err)
		}
	}()

	// auxCallback constructs SolveStatus structs to send to the solveStatus channel,
	// which is in turn consumed by DisplaySolveStatus in the above goroutine.
	//
	// auxCallback is invoked by jsonmessage.DisplayJSONMessagesStream below.
	auxCallback := func(msg jsonmessage.JSONMessage) {
		if msg.ID != "moby.buildkit.trace" {
			return
		}

		var dt []byte
		// ignoring all messages that are not understood
		if err := json.Unmarshal(*msg.Aux, &dt); err != nil {
			// deliberately not logging error
			return
		}
		var resp controlapi.StatusResponse
		if err := (&resp).Unmarshal(dt); err != nil {
			// deliberately not logging error
			return
		}

		// The following lines copy protobuf structs of the buildkit Graph
		// into the buildkit's golang structs for the graph represented by SolveStatus
		s := buildkitClient.SolveStatus{}
		for _, v := range resp.Vertexes {
			s.Vertexes = append(s.Vertexes, &buildkitClient.Vertex{
				Digest:    v.Digest,
				Inputs:    v.Inputs,
				Name:      v.Name,
				Started:   v.Started,
				Completed: v.Completed,
				Error:     v.Error,
				Cached:    v.Cached,
			})
		}
		for _, v := range resp.Statuses {
			s.Statuses = append(s.Statuses, &buildkitClient.VertexStatus{
				ID:        v.ID,
				Vertex:    v.Vertex,
				Name:      v.Name,
				Total:     v.Total,
				Current:   v.Current,
				Timestamp: v.Timestamp,
				Started:   v.Started,
				Completed: v.Completed,
			})
		}
		for _, v := range resp.Logs {
			s.Logs = append(s.Logs, &buildkitClient.VertexLog{
				Vertex:    v.Vertex,
				Stream:    int(v.Stream),
				Data:      v.Msg,
				Timestamp: v.Timestamp,
			})
		}

		solveStatus <- &s
	}

	termFd, isTerm := term.GetFdInfo(jetpackPrinter)
	err := jsonmessage.DisplayJSONMessagesStream(resp.Body, jetpackPrinter, termFd, isTerm, auxCallback)
	return errors.Wrap(err, "failed to print image-push output")
}

func dockerBuildContext(
	plan *BuildPlan,
	dockerfileName string,
) (io.ReadCloser, error) {
	opts := archive.TarOptions{}
	f, err := os.Open(filepath.Join(plan.projectDir, ".dockerignore"))
	if err == nil {
		defer f.Close()
		patterns, err := dockerignore.ReadAll(f)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		opts.ExcludePatterns = lo.Filter(patterns, func(s string, _ int) bool {
			return s != dockerfileName
		})
	}
	return archive.TarWithOptions(plan.projectDir, &opts)
}

// getImageNameAndTag returns a valid docker image name
// and its imageTag is appended.
func getImageNameAndTag(ctx context.Context, opts *BuildOptions) (string, string, error) {
	name := opts.AppName
	name, err := kubevalidate.ToValidName(filepath.Base(name))
	if err != nil {
		return "", "", errors.WithStack(err)
	}

	return name, generateDateImageTag(opts.TagPrefix), nil
}

// This function inspired by https://github.com/hashicorp/waypoint/pull/1937
func initBuildkitSession(
	ctx context.Context,
	dockerClient *dockerclient.Client,
	buildOpts *dockertypes.ImageBuildOptions,
	plan *BuildPlan,
) error {
	if buildOpts.Version == dockertypes.BuilderV1 {
		return nil
	}

	const minDockerVersion = "1.39" // This is only required for buildkit.

	if !versions.GreaterThanOrEqualTo(
		dockerClient.ClientVersion(), minDockerVersion) {
		return errOldDockerAPIVersion
	}

	buildkitSession, err := session.NewSession(ctx, "jetpack", "")
	if err != nil {
		return errors.WithStack(err)
	}
	defer buildkitSession.Close()

	// This env-var is set when sshagent is configured
	if os.Getenv("SSH_AUTH_SOCK") != "" {
		// This allows us to use --mount=type=ssh in the dockerfile
		configs := []sshprovider.AgentConfig{{ID: "default"}}
		if a, err := sshprovider.NewSSHAgentProvider(configs); err != nil {
			// TODO(Landau), show good user error if ssh keys are not added to agent
			return errors.WithStack(err)
		} else {
			buildkitSession.Allow(a)
		}
	} else {
		jetlog.Logger(ctx).IndentedPrintln("Warning: Did not find SSH_AUTH_SOCK env var. " +
			"Skipping ssh agent provider for docker buildkit.")
	}

	if plan.buildOpts.RemoteCache {
		c, err := decodeCredentials(plan.buildOpts.RepoConfig)
		if err != nil {
			return errors.WithStack(err)
		}
		buildkitSession.Allow(authprovider.NewDockerAuthProvider(authprovider.NewConfig(
			plan.buildOpts.GetRepoHost(),
			c.Username,
			c.Password,
		)))
	}

	dialSession := func(
		ctx context.Context,
		proto string,
		meta map[string][]string,
	) (net.Conn, error) {
		return dockerClient.DialHijack(ctx, "/session", proto, meta)
	}

	go func() {
		err = buildkitSession.Run(ctx, dialSession)
		if err != nil {
			panic(err) // Is there a better way to handle this?
		}
	}()

	buildOpts.SessionID = buildkitSession.ID()

	return nil
}

func getDockerBuilderVersion(ctx context.Context, path string) (dockertypes.BuilderVersion, error) {
	// Respect env-var
	if os.Getenv("DOCKER_BUILDKIT") == "1" {
		jetlog.Logger(ctx).IndentedPrintln("Detecting DOCKER_BUILDKIT=1. Using Buildkit Docker builder.")
		return dockertypes.BuilderBuildKit, nil
	}
	if os.Getenv("DOCKER_BUILDKIT") == "0" {
		jetlog.Logger(ctx).IndentedPrintln("Detecting DOCKER_BUILDKIT=0. Using v1 Docker builder.")
		return dockertypes.BuilderV1, nil
	}

	// Fallback to reading the Dockerfile
	content, err := os.ReadFile(filepath.Join(path, "Dockerfile"))
	if err != nil {
		jetlog.Logger(ctx).IndentedPrintf("No detecting Dockerfile at %s. Using v1 Docker builder.\n", path)
		return dockertypes.BuilderV1, errors.WithStack(err)
	}
	trimmed := strings.TrimSpace(string(content))

	if strings.HasPrefix(trimmed, "# syntax") {
		jetlog.Logger(ctx).IndentedPrintln("Detecting # syntax in Dockerfile. Using Buildkit Docker builder.")
		return dockertypes.BuilderBuildKit, nil
	}

	if strings.Contains(trimmed, "RUN --mount") {
		jetlog.Logger(ctx).IndentedPrintln("Detecting RUN --mount in Dockerfile. Using Buildkit Docker builder.")
		return dockertypes.BuilderBuildKit, nil
	}

	jetlog.Logger(ctx).IndentedPrintln("Using v1 Docker builder.")
	return dockertypes.BuilderV1, nil
}

// DockerCleanup deletes all docker images that are not the latest based on timestamp.
// This will only delete the images that belong to the current project.
func DockerCleanup(ctx context.Context, labelIdentifier string) error {
	cli, err := dockerclient.NewClientWithOpts(
		dockerclient.FromEnv,
		dockerclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return errors.WithStack(err)
	}

	filters := filters.NewArgs()
	filters.Add("label", fmt.Sprintf("%s=%s", dockerProjectIdLabel, labelIdentifier))

	imgs, err := cli.ImageList(ctx, dockertypes.ImageListOptions{
		Filters: filters,
	})
	if err != nil {
		return errors.WithStack(err)
	}

	var latestTimestamp int64 = -1
	for _, d := range imgs {
		if d.Labels[dockerProjectIdLabel] == labelIdentifier {
			if d.Created > latestTimestamp {
				latestTimestamp = d.Created
			}
		}
	}

	if latestTimestamp == -1 {
		return nil
	}

	for _, d := range imgs {
		if d.Labels[dockerProjectIdLabel] == labelIdentifier && d.Created != latestTimestamp {
			_, err = cli.ImageRemove(
				ctx,
				d.ID,
				dockertypes.ImageRemoveOptions{PruneChildren: true, Force: true},
			)

			return errors.WithStack(err)
		}
	}

	return nil
}

type imageRepoCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func decodeCredentials(c provider.RepoConfig) (*imageRepoCredentials, error) {
	if c == nil || c.GetCredentials() == "" {
		return nil, nil
	}
	s, err := base64.StdEncoding.DecodeString(c.GetCredentials())
	if err != nil {
		return nil, errors.WithStack(err)
	}
	creds := &imageRepoCredentials{}
	return creds, json.Unmarshal(s, creds)
}
