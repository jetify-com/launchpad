package launchpad

import (
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	dockertypes "github.com/docker/docker/api/types"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/fatih/color"
	"github.com/moby/term"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"go.jetpack.io/launchpad/goutil"
	"go.jetpack.io/launchpad/goutil/errorutil"
	"go.jetpack.io/launchpad/padcli/hook"
	"go.jetpack.io/launchpad/padcli/provider"
	"go.jetpack.io/launchpad/pkg/jetlog"
	"go.jetpack.io/launchpad/pkg/kubevalidate"
)

type registryHost string

// TODO nit: sort in abc order. Not doing now to keep code-reviewable.
const (
	// unknownRegistryHost a catch-all for a docker-registry provider we do not
	// do any special handling for. The requirement is that this provider supports
	// create-repo-upon-image-push (unlike AWS and GCP), OR the user can manually
	// create the repository first.
	//
	// Digital Ocean is an example of such a provider.
	unknownRegistryHost         registryHost = "unknown"
	jetpackProvidedRegistryHost registryHost = "jetpackProvided"
	dockerHubRegistryHost       registryHost = "dockerHub"
	gcpRegistryHost             registryHost = "gcp"
	awsRegistryHost             registryHost = "aws"
)

func (h registryHost) usesCreateOnPush() bool {
	return h == gcpRegistryHost ||
		h == dockerHubRegistryHost ||
		h == unknownRegistryHost
}

const (
	dockerHubRegistryUri = "docker.io"

	// Docker hub credentials are saved in ~/.docker/config.json indexed by this key.
	// the "serverAddress" name comes from its usage in the type.AuthConfig struct
	// https://github.com/docker/cli/blob/master/cli/config/types/authconfig.go#L14
	dockerHubRegistryServerAddress = "https://index.docker.io/v1/"
)

// Regex derived from https://github.com/aws/aws-sam-cli/blob/2592135cd21cb5fb1559e866cce2d5383ee49536/samcli/lib/package/regexpr.py
// This matches private ecr repos and public one.
var ecrURIRegex = regexp.MustCompile(
	`((^[a-zA-Z0-9][a-zA-Z0-9-_]*)\.dkr\.ecr\.([a-zA-Z0-9][a-zA-Z0-9-_]*)\.amazonaws\.com(\.cn)?` +
		`\/(?:[a-z0-9]+(?:[._-][a-z0-9]+)*\/)*[a-z0-9]+(?:[._-][a-z0-9]+)*)|public\.ecr\.aws\/.*`,
)

// Regex based on https://cloud.google.com/artifact-registry/docs/docker/names
// as well as https://cloud.google.com/container-registry/docs/using-with-google-cloud-platform
var gcrURIRegex = regexp.MustCompile(
	`((gcr\.io)|((us|eu|asia)?\.gcr\.io)|(([a-zA-Z0-9\-_]*)\-docker\.pkg\.dev))\/[a-zA-Z0-9\-_\/\:]*`,
)

type ImageRegistry struct {
	awsCfg *aws.Config

	// host specifies the kind of registry host we are using
	host registryHost

	dockerCredentials string

	// Uri is the location of the registry
	// e.g. 984256416385.dkr.ecr.us-west-2.amazonaws.com
	// e.g. localhost:8080
	uri string
}

type PublishOptions struct {
	AnalyticsProvider    provider.Analytics
	AWSCredentials       aws.CredentialsProvider // Required to create ECR repos
	ImageRepoCredentials string

	// ImageRegistryWithRepo is <registry-uri>/<repository-path>
	ImageRegistryWithRepo string
	LifecycleHook         hook.LifecycleHook
	LocalImages           []*LocalImage
	Region                string
	TagPrefix             string
}

type PublishImagePlan struct {
	localImage      *LocalImage
	remoteImageName string
	remoteImageTag  string
}

type PublishPlan struct {
	images    []*PublishImagePlan
	imageRepo string
	// registry has information about the image's registry. This may be nil if
	// no image-registry was specified by the user.
	registry *ImageRegistry
}

type PublishOutput struct {
	Duration        time.Duration
	registryHost    registryHost
	publishedImages map[string]string
}

func (p *PublishPlan) imageRepository() string {
	return p.imageRepo
}

func (p *PublishImagePlan) remoteImageNameWithTag() string {
	return fmt.Sprintf("%s:%s", p.remoteImageName, p.remoteImageTag)
}

func (p *PublishImagePlan) remoteImageNameWithLatestTag() string {
	return fmt.Sprintf("%s:%s", p.remoteImageName, "latest")
}

func (ir *ImageRegistry) GetHost() registryHost {
	return ir.host
}

func (o *PublishOutput) DidPublish() bool {
	if o == nil {
		return false
	}
	return len(o.publishedImages) > 0
}

func (o *PublishOutput) PublishedImages() map[string]string {
	if o == nil {
		return nil
	}
	return o.publishedImages
}

func (do *PublishOutput) SetDuration(d time.Duration) {
	if do != nil {
		do.Duration = d
	}
}

func (o *PublishOutput) RegistryHost() string {
	if o == nil {
		return ""
	}
	return string(o.registryHost)
}

func getRemoteRegistryInfo(
	ctx context.Context,
	pubOpts *PublishOptions,
) (*struct {
	cloudRegion    string
	repositoryPath string
	registry       *ImageRegistry
}, error) {
	var registry *ImageRegistry
	var repositoryPath string
	var cloudRegion string

	// If options has all credentials, use them. Otherwise fall back to local
	// docker credentials.
	// TODO: We may be able to unify this logic.
	if pubOpts.AWSCredentials != nil && pubOpts.ImageRepoCredentials != "" {

		if pubOpts.ImageRepoCredentials == "" {
			return nil, errors.New("expect ImageRepoConfig in PublishOptions for jetpack provided registry")
		}

		registryUri := strings.Split(pubOpts.ImageRegistryWithRepo, "/")[0]
		awsConfig, err := config.LoadDefaultConfig(
			ctx,
			config.WithCredentialsProvider(pubOpts.AWSCredentials),
			config.WithRegion(pubOpts.Region),
		)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		registry = &ImageRegistry{
			awsCfg:            &awsConfig,
			host:              jetpackProvidedRegistryHost,
			dockerCredentials: pubOpts.ImageRepoCredentials,
			uri:               registryUri,
		}
		if err != nil {
			return nil, errors.Wrap(err, "failed to get authenticated Ecr Registry")
		}

		repositoryPath = strings.TrimPrefix(
			pubOpts.ImageRegistryWithRepo,
			registryUri+"/",
		)

		jetlog.Logger(ctx).IndentedPrintf("Publishing with image registry: %s/%s\n", registry.uri, repositoryPath)
	} else {

		jetlog.Logger(ctx).IndentedPrintf(
			"Publishing with image registry: %s\n",
			pubOpts.ImageRegistryWithRepo,
		)
		var err error
		registry, repositoryPath, err = imageRegistryAndRepository(ctx, pubOpts.ImageRegistryWithRepo)
		if err != nil {
			return nil, errors.WithStack(err)
		}
	}

	registryInfo := struct {
		cloudRegion    string
		repositoryPath string
		registry       *ImageRegistry
	}{
		cloudRegion,
		repositoryPath,
		registry,
	}
	return &registryInfo, nil
}

func (p *Pad) publishSingleImage(
	ctx context.Context,
	registry *ImageRegistry,
	plan *PublishImagePlan,
) error {
	if registry == nil {
		jetlog.Logger(ctx).IndentedPrintln("Skipping publish-step. No image registry to push to.")
		return nil
	}

	dockerClient, err := dockerclient.NewClientWithOpts(
		dockerclient.FromEnv,
		dockerclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return errors.WithStack(err)
	}

	err = p.tagAndPushImage(
		ctx,
		registry,
		plan,
		dockerClient,
		plan.remoteImageNameWithTag(),
		dockerWriter{},
	)
	if err != nil {
		return errors.WithStack(err)
	}
	// This is clobbering latest which is not ideal. If we push to different repos
	// then this would be fixed.
	return errors.WithStack(
		p.tagAndPushImage(
			ctx,
			registry,
			plan,
			dockerClient,
			plan.remoteImageNameWithLatestTag(),
			io.Discard,
		),
	)
}

func (p *Pad) tagAndPushImage(
	ctx context.Context,
	registry *ImageRegistry,
	plan *PublishImagePlan,
	dockerClient *dockerclient.Client,
	remoteNameWithTag string,
	out io.Writer,
) error {

	inspect, _, err := dockerClient.ImageInspectWithRaw(ctx, plan.localImage.String())
	if err != nil {
		return errors.WithStack(err)
	}

	if inspect.Architecture != "amd64" {
		color.New(color.FgRed).Fprintf(
			jetlog.Logger(ctx),
			"\n\n\n###### Warning ######\n\n"+
				"Image %s is not an amd64 image. We will still publish it and try to "+
				"use it, but it will likely not work. Please build your image with the"+
				" amd64 architecture.\n\n#####################\n\n",
			plan.localImage.String(),
		)
	}

	jetlog.Logger(ctx).IndentedPrintf(
		"docker tag %s %s\n",
		plan.localImage,
		remoteNameWithTag,
	)

	if err = dockerClient.ImageTag(
		ctx,
		plan.localImage.String(),
		remoteNameWithTag,
	); err != nil {
		if strings.Contains(err.Error(), "No such image") {
			return errorutil.AddUserMessagef(
				err,
				"Image %s not found. If you want to publish a remote image, please pull it first.",
				plan.localImage.String(),
			)
		}
		return errors.WithStack(err)
	}

	jetlog.Logger(ctx).IndentedPrintf("docker push %s\n", remoteNameWithTag)

	opts := dockertypes.ImagePushOptions{
		// Use the docker-credentials string
		//
		// Fallback:
		// Must return an arbitrary string, in this case "123". Otherwise,
		// one gets an error saying: "Bad parameters and missing X-Registry-Auth: EOF".
		// This is needed (for example) for a docker registry running on localhost
		// https://stackoverflow.com/a/46239427
		RegistryAuth: goutil.Coalesce(registry.dockerCredentials, "123"),
	}

	// Print the output during image-push:
	// thank you: https://stackoverflow.com/a/58742917
	termFd, isTerm := term.GetFdInfo(os.Stderr)
	maxTries := 3
	delay := time.Duration(5)
	_, _, err = lo.AttemptWithDelay(
		maxTries,
		delay*time.Second,
		func(i int, _ time.Duration) error {
			pusher, err := dockerClient.ImagePush(
				ctx,
				remoteNameWithTag,
				opts,
			)
			if err == nil {
				defer pusher.Close()
				err = jsonmessage.DisplayJSONMessagesStream(
					pusher,
					out,
					termFd,
					isTerm,
					nil,
				)
			}
			// This error could be ImagePush or DisplayJSONMessagesStream
			if err != nil && i < maxTries-1 {
				p.errorLogger.CaptureException(err)
				jetlog.Logger(ctx).BoldPrintf(
					"Error pushing image. Waiting %d seconds and trying again\n",
					delay,
				)
				jetlog.Logger(ctx).IndentedPrintf("Error is: %s\n", err.Error())
			}
			return errors.WithStack(err)
		})

	// Returning the original error as part of user error because the original error gives
	// better context on what went wrong to the user.
	// Docker image push can fail for many reasons and a custom user error can't predict all cases.
	return errorutil.AddUserMessagef(
		err,
		"Failed to push to image registry. Please check your internet connection and try again.",
	)
}

func imageRegistryAndRepository(
	ctx context.Context,
	imageRegistryWithRepo string,
) (*ImageRegistry, string, error) {
	regUriParts := strings.Split(imageRegistryWithRepo, "/")

	registryUri := regUriParts[0]
	// janky check for isDomain; I couldn't find a good library.
	// check for a period to indicate a subdomain.
	//
	// For a registryUri like "savil/jetpack-demo", the registryUri is
	// empty-string implying implicitly that the registryUri is "docker.io".
	if !strings.Contains(registryUri, ".") {
		registryUri = ""
	}
	registryType, err := registryTypeFromUri(registryUri)
	if err != nil {
		return nil, "", errors.WithStack(err)
	}

	var registry *ImageRegistry
	if registryType == awsRegistryHost {
		var err error
		registry, err = getAuthenticatedEcrRegistryWithDefaultConfig(ctx, registryUri)
		if err != nil {
			return nil, "", errors.WithStack(err)
		}
	} else if registryType.usesCreateOnPush() {

		addr := registryUri
		if registryType == dockerHubRegistryHost {
			addr = dockerHubRegistryServerAddress
		}
		creds, err := credentialsFromDockerCredentialStore(addr)
		if err != nil {
			return nil, "", errors.Wrapf(err,
				"failed to get creds from docker credential store for registryType: %s registryUri: %s",
				registryType,
				registryUri,
			)
		}

		uri := registryUri
		if registryType == dockerHubRegistryHost {
			uri = dockerHubRegistryUri
		}
		registry = &ImageRegistry{
			host:              registryType,
			uri:               uri,
			dockerCredentials: creds,
		}
	} else {
		return nil, "", errors.Errorf("unsupported registryType: %s", registryType)
	}

	// Resolve the image-repository:
	repository := ""
	if len(regUriParts) > 1 {
		repository = strings.Join(regUriParts[1:], "/")
	}

	return registry, repository, nil
}

func registryTypeFromUri(uri string) (registryHost, error) {
	if uri == "" || strings.Contains(uri, "hub.docker") || strings.Contains(uri, "docker.io") {
		return dockerHubRegistryHost, nil
	}

	if ecrURIRegex.MatchString(uri) {
		return awsRegistryHost, nil
	}

	if gcrURIRegex.MatchString(uri) {
		return gcpRegistryHost, nil
	}

	return unknownRegistryHost, nil
}

func (p *Pad) publishDockerImage(
	ctx context.Context,
	opts *PublishOptions,
) (*PublishOutput, error) {

	plan, err := makePublishPlan(ctx, opts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to make build plan")
	}

	err = validatePublishPlan(plan)
	if err != nil {
		return nil, err
	}

	err = p.executePublishPlan(ctx, plan)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute build plan")
	}

	published := map[string]string{}
	for _, image := range plan.images {
		published[image.localImage.String()] = image.remoteImageNameWithTag()
	}

	return &PublishOutput{
		publishedImages: published,
		registryHost:    plan.registry.GetHost(),
	}, nil
}

func makePublishPlan(
	ctx context.Context,
	opts *PublishOptions,
) (*PublishPlan, error) {
	registryInfo, err := getRemoteRegistryInfo(
		ctx,
		opts,
	)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	plan := &PublishPlan{
		images: lo.Map(
			opts.LocalImages,
			func(l *LocalImage, _ int) *PublishImagePlan {
				return &PublishImagePlan{
					localImage:      l,
					remoteImageName: opts.ImageRegistryWithRepo,
					remoteImageTag: generateDateImageTag(
						// Add deterministic slug to ensure no collisions when publishing
						// multiple images. Ideally these go to different repos but
						// that requires improving the permission model (already a TODO)
						opts.TagPrefix + kubevalidate.DeterministicSlug(l.String()) + "-",
					),
				}
			},
		),
		imageRepo: registryInfo.repositoryPath,
		registry:  registryInfo.registry,
	}
	return plan, nil
}

func validatePublishPlan(plan *PublishPlan) error {
	if plan.registry.host == gcpRegistryHost {
		// Background Context
		// As an example, consider an image at:
		// us-central1-docker.pkg.dev/jetpack-dev/savil-cluster-test-2/py-dockerfile:234234
		//
		// Docker calls its parts:
		// - repository: us-central1-docker.pkg.dev/jetpack-dev/savil-cluster-test-2/py-dockerfile
		// - tag: 234234
		//
		// Google Artifact Registry calls its parts:
		// - registry: us-central1-docker.pkg.dev/
		// - repository: jetpack-dev/savil-cluster-test-2
		// - image name:  py-dockerfile
		// - tag: 234234
		//
		// GCP users may mistake the --image-repository flag, or jetconfig.imageRepository
		// to refer to just "jetpack-dev/savil-cluster-test-2" or "us-central1-docker.pkg.dev/jetpack-dev/savil-cluster-test-2"
		//
		// So we add this validation rule to catch this error
		for _, imagePlan := range plan.images {
			parts := strings.Split(imagePlan.remoteImageName, "/")

			// Ensure:
			// 1. 4 or more parts.
			// 2. None of the parts are empty.
			if len(parts) < 4 || lo.SomeBy(parts, func(p string) bool { return len(strings.TrimSpace(p)) == 0 }) {

				const gcpDocsFormatURL = "https://cloud.google.com/artifact-registry/docs/docker/names#containers"
				return errorutil.NewUserErrorf(
					"The image repository you have used has an invalid format for Google Artifact Registry. \n"+
						" - Please note that what docker refers to as \"image repository\" corresponds to the \"image"+
						" name\" in Google terminology. \n"+
						" - Please refer to the docs at %s for the image name format to use.",
					gcpDocsFormatURL,
				)
			}
		}
	}
	return nil
}

func (p *Pad) executePublishPlan(
	ctx context.Context,
	plan *PublishPlan,
) error {
	err := ensureRepositoryExistsOnRegistry(ctx, plan)
	if err != nil {
		return errors.Wrap(err, "failed to execute pre-publish plan")
	}

	for _, imagePlan := range plan.images {
		err = p.publishSingleImage(ctx, plan.registry, imagePlan)
		if err != nil && strings.Contains(err.Error(), "no active session") {
			jetlog.Logger(ctx).Print(
				"\nERROR: No active Buildkit session found. Waiting 5 seconds and " +
					"trying again\n",
			)
			time.Sleep(5 * time.Second)
			err = p.publishSingleImage(ctx, plan.registry, imagePlan)
		}
	}

	return errors.Wrap(err, "failed to publish to registry")
}

func ensureRepositoryExistsOnRegistry(
	ctx context.Context,
	plan *PublishPlan,
) error {
	if plan.registry == nil {
		jetlog.Logger(ctx).IndentedPrintln("No registry specified, so skipping ensuring the repository exists.")
		return nil
	}

	if plan.registry.host == jetpackProvidedRegistryHost ||
		plan.registry.host == awsRegistryHost {
		err := createEcrRepository(ctx, plan)
		if err != nil {
			return errors.Wrap(err, "failed to create ECR imageRepository")
		}
	} else if plan.registry.host == gcpRegistryHost {
		err := createGcpRepository(ctx, plan)
		if err != nil {
			return errors.Wrap(err, "failed to create GCP imageRepository")
		}
	}
	return nil
}
