package launchpad

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	dockerclient "github.com/docker/docker/client"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"github.com/spf13/afero"
	"go.jetpack.io/launchpad/goutil/errorutil"
	"go.jetpack.io/launchpad/padcli/hook"
	"go.jetpack.io/launchpad/padcli/jetconfig"
	"go.jetpack.io/launchpad/padcli/provider"
	"go.jetpack.io/launchpad/pkg/docker"
	"go.jetpack.io/launchpad/pkg/jetlog"
	"go.jetpack.io/launchpad/pkg/kubevalidate"
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
				"https://github.com/jetify/project-templates/blob/main/api/Dockerfile.\n" +
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

	imageBuildOptions := docker.BuildOpts{
		BuildArgs: lo.MapValues(plan.buildOpts.BuildArgs, func(val, _ string) *string {
			return &val
		}),
		Dockerfile: dockerfile,
		Platform:   plan.buildOpts.Platform,
		Tags:       []string{plan.image.String()},
		Labels:     plan.imageLabels,
	}

	return docker.Build(ctx, filepath.Dir(plan.dockerfilePath), imageBuildOptions)
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
