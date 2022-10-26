package command

import (
	"context"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/fatih/color"
	"github.com/joho/godotenv"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"go.jetpack.io/envsec"
	"go.jetpack.io/launchpad/goutil"
	"go.jetpack.io/launchpad/goutil/errorutil"
	"go.jetpack.io/launchpad/launchpad"
	"go.jetpack.io/launchpad/padcli/command/jflags"
	"go.jetpack.io/launchpad/padcli/jetconfig"
	"go.jetpack.io/launchpad/padcli/k8s"
	"go.jetpack.io/launchpad/padcli/provider"
	"go.jetpack.io/launchpad/pkg/jetlog"
	"go.jetpack.io/launchpad/pkg/kubevalidate"
)

const (
	minReplicaFlag       = "min-replicas"
	mountSecretFileFlag  = "mount-secret-file"
	mountSecretFilesFlag = "mount-secret-files"
	publicFlag           = "public"
	envOverrideFlag      = "env-override"
)

// TODO: move all flag messages and flag names to the same place to re-use
const envOverrideFlagMsg = "Specifies Env file(s) to use. " +
	"This makes Launchpad ignore environment variables set in `launchpad env`. "

// TODO: Move to a common file
var green = color.New(color.FgGreen)

type publishOptions struct {
	// ImageRepo is <registry-uri>/<repository-path>
	ImageRepo string
}

type upOptions struct {
	jflags.Common
	deployOptions
	embeddedBuildOptions
	publishOptions
}

// set here for test
var upOpts = &upOptions{}

func upCmd() *cobra.Command {

	opts := upOpts

	upCmd := &cobra.Command{
		Use:     "up [path]",
		Short:   "builds and deploys app",
		Args:    cobra.MaximumNArgs(1),
		PreRunE: validateDeployUptions(&opts.deployOptions),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := cmdOpts.AuthProvider().Identify(cmd.Context())
			if err != nil {
				return errors.WithStack(err)
			}

			absPath, err := projectDir(args)
			if err != nil {
				return errors.WithStack(err)
			}
			jetCfg, err := loadOrInitConfigFromFileSystem(ctx, cmd, args)
			if err != nil {
				return errors.WithStack(err)
			}

			cluster, err := cmdOpts.ClusterProvider().Get(ctx, opts.DefaultedCluster(jetCfg))
			if err != nil {
				return errors.WithStack(err)
			}

			repoConfig, err := cmdOpts.RepositoryProvider().Get(ctx, cluster)
			if err != nil {
				return errors.WithStack(err)
			}

			store, err := newEnvStore(ctx, cmdOpts.EnvSecProvider())
			if err != nil {
				return errors.WithStack(err)
			}

			pad := launchpad.NewPad(cmdOpts.ErrorLogger())
			do, bpdErr := buildPublishAndDeploy(
				ctx,
				pad,
				cmd,
				jetCfg,
				&opts.embeddedBuildOptions,
				&opts.deployOptions,
				goutil.Coalesce(opts.ImageRepo, jetCfg.ImageRepository),
				absPath,
				cluster,
				repoConfig,
				store,
			)

			if bpdErr != nil {
				// NOTE: we should check that bpdError is a deploy error and not a build or publish error
				if errors.Is(bpdErr, launchpad.ErrPodContainerError) {
					return bpdErr
				}
				if err := pad.TailLogsOnErr(ctx, cluster.GetKubeContext(), do); err != nil {
					return errors.Wrap(bpdErr, "failed to tail logs")
				}
				// TODO(Savil): Waiting 2 seconds is not reliable. We need to actually
				// listen to the an event that tells us the logs have been displayed
				secondsToWait := 2
				fmt.Printf("Waiting %d seconds for all logs to be shown\n", secondsToWait)
				time.Sleep(time.Duration(secondsToWait) * time.Second)
				return errorutil.AddUserMessagef(
					bpdErr,
					"Deploy was not successful. See pod logs for more details.",
				)
			}

			return printUpSuccess(ctx, do, cluster)
		},

		PostRun: func(cmd *cobra.Command, args []string) {
			if err := cleanupPreviousBuildsPostRun(cmd, args); err != nil {
				cmdOpts.ErrorLogger().CaptureException(err)
				return
			}
		},
	}

	registerUpFlags(upCmd, opts)
	return upCmd
}

func registerUpFlags(cmd *cobra.Command, opts *upOptions) {
	jflags.RegisterCommonFlags(cmd, &opts.Common)
	registerDeployFlags(cmd, &opts.deployOptions)
	registerEmbeddedBuildFlags(cmd, &opts.embeddedBuildOptions)
	registerPublishFlags(cmd, &opts.publishOptions)
}

func registerPublishFlags(cmd *cobra.Command, opts *publishOptions) {
	cmd.Flags().StringVarP(
		&opts.ImageRepo,
		"image-repository",
		"i",
		"",
		imageRepositoryFlagHelpMsg,
	)
}

func registerDeployFlags(cmd *cobra.Command, opts *deployOptions) {

	cmd.Flags().StringVar(
		&opts.App.SetValues,
		"helm.app.set",
		"",
		"args passed to helm --set option for app (can specify multiple or separate values with commas: key1=val1,key2=val2)",
	)

	cmd.Flags().StringSliceVar(
		&opts.App.ValueFiles,
		"helm.app.values",
		[]string{},
		"args passed to helm --values option for app. See helm docs for more info.",
	)

	cmd.Flags().StringVar(
		&opts.App.ChartLocation,
		"helm.app.chart-location",
		"",
		"custom location for app chart",
	)
	cmd.Flags().StringVar(
		&opts.App.Name,
		"helm.app.name",
		"",
		"App install name",
	)

	cmd.Flags().StringVar(
		&opts.Runtime.SetValues,
		"helm.runtime.set",
		"",
		"args passed to helm --set option for runtime (can specify multiple or separate values with commas: key1=val1,key2=val2)",
	)
	cmd.Flags().StringVar(
		&opts.Runtime.ChartLocation,
		"helm.runtime.chart-location",
		"",
		"custom location for runtime chart",
	)

	cmd.Flags().StringVarP(
		&opts.Namespace,
		"namespace",
		"n",
		"",
		"K8s Namespace",
	)

	cmd.Flags().IntVar(
		lo.ToPtr(lo.Empty[int]()),
		minReplicaFlag,
		0, // Should be in sync with helm/app/values.yaml
		"Minimum number of replicas. Alias for "+
			"--helm.app.set autoscaling.minReplicas=n. Setting "+
			"min-replica count to zero will skip the creation of a deployment and "+
			"service. Setting anything above zero will enable pod autoscalling",
	)
	_ = cmd.Flags().MarkHidden(minReplicaFlag)
	_ = cmd.Flags().MarkDeprecated(
		minReplicaFlag,
		"This flag is deprecated and no longer does anything. min replicas is "+
			"always 1",
	)

	cmd.Flags().StringSliceVar(
		&opts.SecretFilePaths,
		mountSecretFilesFlag,
		[]string{},
		"Mount a set of secret files : "+
			"Each mounted entry has a name derived from the base file name",
	)

	cmd.Flags().BoolVar(
		lo.ToPtr(lo.Empty[bool]()),
		publicFlag,
		false,
		"Expose service publicly using API gateway.",
	)
	_ = cmd.Flags().MarkHidden(publicFlag)
	_ = cmd.Flags().MarkDeprecated(
		publicFlag,
		"This flag is deprecated and no longer does anything. web apps are always public",
	)

	cmd.Flags().StringVar(
		&opts.envOverrideFile,
		envOverrideFlag,
		"",
		envOverrideFlagMsg,
	)
	// Made env-override file flag hidden temporarily until we decide on concrete approach
	// on whether override merges with launchpad env or skips it
	_ = cmd.Flags().MarkHidden(envOverrideFlag)

	cmd.Flags().StringVar(
		&opts.execQualifiedSymbol,
		"exec",
		"",
		"Execute a single cronjob or jetroutine for a given function.",
	)

	cmd.Flags().BoolVar(
		&opts.ReinstallOnHelmUpgradeError,
		"reinstall-on-error",
		false,
		"Attempt reinstall of helm charts if upgrade fails. This flag does not "+
			"affect initial installs. Warning: reinstalling will cause loss of "+
			"revision history and potential downtime.",
	)
}

// TODO: lets figure out a way to reduce this long list of parameters
func buildPublishAndDeploy(
	ctx context.Context,
	pad launchpad.LaunchPad,
	cmd *cobra.Command,
	jetCfg *jetconfig.Config,
	buildOpts *embeddedBuildOptions,
	deployOpts *deployOptions,
	imageRepoOverride string,
	projPath string,
	cluster provider.Cluster,
	repoConfig provider.RepoConfig,
	store envsec.Store,
) (*launchpad.DeployOutput, error) {

	_, buildOutput, err := execLaunchpadBuild(
		ctx, pad, buildOpts, jetCfg, cluster, projPath, repoConfig, "1/3" /*stepInfo*/)
	if err != nil {
		return nil, errors.Wrap(err, "Error building image")
	}

	_, pubOutput, err := execLaunchpadPublish(
		ctx, pad, buildOutput, imageRepoOverride, repoConfig, cluster, jetCfg, "2/3" /*stepInfo*/)
	if err != nil {
		return nil, errors.Wrap(err, "Error publishing images")
	}

	jetlog.Logger(ctx).HeaderPrintf("Step 3/3: Deploying project to your cluster")
	lpDeployOpts, err := makeDeployOptions(
		ctx, cmd, jetCfg, pubOutput, buildOutput, deployOpts, projPath, cluster, store,
	)
	if err != nil {
		return nil, errors.Wrap(err, "Error deploying")
	}

	do, err := pad.Deploy(ctx, lpDeployOpts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to deploy image")
	}
	jetlog.Logger(ctx).HeaderPrintf(
		"[Done] App deployed in %s",
		do.Duration.Truncate(time.Millisecond*100),
	)
	return do, nil
}

func printUpSuccess(
	ctx context.Context,
	do *launchpad.DeployOutput,
	c provider.Cluster,
) error {
	jetlog.Logger(ctx).Println(green.Sprintf(
		"App deployed to namespace \"%s\"",
		do.Namespace,
	))

	// TODO(Landau) This gets more complicated when we add internal services.
	// Consider adding ambassador.enabled value.
	values := do.Releases[launchpad.AppChartName].Config
	if fmt.Sprintf("%v", values["replicaCount"]) == "0" {
		return nil
	}

	if c.IsLocal() {
		name := values["jetpack"].(map[string]any)["instanceName"].(string) +
			// Ugh, this makes me so sad
			"-" + launchpad.AppChartName
		port, err := k8s.ServiceNodePort(ctx, name, do.Namespace, c.GetKubeContext())
		if err != nil {
			return errors.Wrap(err, "failed to get service node port")
		}
		jetlog.Logger(ctx).Println(
			green.Sprintf("App reachable at http://localhost:%d", port),
		)
		return nil
	}

	if amby, ok := values["ambassador"].(map[string]any); ok {
		host := amby["hostname"].(string)
		if host != "" {
			jetlog.Logger(ctx).Println(green.Sprintf("App reachable at https://%s", host))
		}
	}

	return nil
}

func readEnvVariables(
	projectPath string,
	envFile string,
	encodeValues bool,
) (map[string]string, error) {
	if !filepath.IsAbs(envFile) {
		envFile = filepath.Join(projectPath, envFile)
	}

	vars, err := godotenv.Read(envFile)
	if err != nil {
		return nil, errorutil.ConvertToUserError(err)
	}
	var envVars = map[string]string{}
	// encoded values are used for Kubernetes deployments
	if encodeValues {
		for key, value := range vars {
			envVars[key] = base64.StdEncoding.EncodeToString([]byte(value))
		}
	} else { // non-encoded values are used for launchpad local
		envVars = vars
	}
	return envVars, nil
}

func validateDeployUptions(opts *deployOptions) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		// TODO: this should load config, merge it into flags and then validate.
		// TODO: prevent env-override and environment=prod flags together
		err := validateNamespace(opts.Namespace)
		if err != nil {
			return errors.WithStack(err)
		}
		return nil
	}
}

func validateNamespace(namespace string) error {
	// namespace follows the same RFC1123 standard as subdomain
	if namespace != "" && !kubevalidate.IsValidRFC1123Name(namespace) {
		msg := "Invalid value for namespace %s.\n" +
			"A lowercase RFC 1123 label must consist of:\n" +
			" - alphanumeric characters\n" +
			" - or '-' with starting and ending with an alphanumeric character.\n"
		return errorutil.NewUserErrorf(msg, namespace)
	}
	return nil
}

func makePublishOptions(
	imageRepoOverride string,
	repoConfig provider.RepoConfig,
	buildOutput *launchpad.BuildOutput,
	config *jetconfig.Config,
) (*launchpad.PublishOptions, error) {
	imageRegistryWithRepo := ""
	if imageRepoOverride != "" {
		imageRegistryWithRepo = imageRepoOverride
		// This means anytime an override repo is set we ignore mc credentials.
		// We could improve this by having mc return an ECR path that the user has
		// access to and checking if the override repo is part of that path.
		repoConfig = nil
	} else if repoConfig != nil {
		suffix, err := kubevalidate.ToValidName(config.GetProjectNameWithSlug())
		if err != nil {
			return nil, err
		}
		imageRegistryWithRepo = repoConfig.GetImageRepoPrefix() + "/" + suffix
	} else {
		return nil, errorutil.NewUserError(
			"No image repo specified. Please use --image-repository flag or " +
				"imageRepository field in launchpad.yaml or jetconfig.yaml to specify",
		)
	}

	localImagesToPublish := []*launchpad.LocalImage{}
	if buildOutput.DidBuildUsingDockerfile() {
		localImagesToPublish = append(localImagesToPublish, buildOutput.Image)
	}

	for _, service := range config.Builders() {
		if service.ShouldPublish() {
			localImagesToPublish = append(
				localImagesToPublish,
				launchpad.NewLocalImage(service.GetImage()),
			)
		}
	}

	opts := &launchpad.PublishOptions{
		ImageRegistryWithRepo: imageRegistryWithRepo,
		LifecycleHook:         cmdOpts.Hooks().Publish,
		LocalImages:           localImagesToPublish,
		TagPrefix:             cmdOpts.RootFlags().Env().ImageTagPrefix(),
	}

	if repoConfig != nil {
		awsCreds, _ := repoConfig.GetCloudCredentials().(aws.CredentialsProvider)
		opts.AWSCredentials = awsCreds
		opts.ImageRepoCredentials = repoConfig.GetCredentials()
		opts.Region = repoConfig.GetRegion()
	}

	return opts, nil
}
