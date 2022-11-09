package command

import (
	"context"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.jetpack.io/launchpad/launchpad"
	"go.jetpack.io/launchpad/padcli/command/jflags"
	"go.jetpack.io/launchpad/padcli/jetconfig"
	"go.jetpack.io/launchpad/padcli/provider"
	"go.jetpack.io/launchpad/pkg/jetlog"
	"go.jetpack.io/launchpad/pkg/kubevalidate"
)

const (
	localImageFlag = "local-image"
)

type embeddedBuildOptions struct {
	Platform    string
	BuildArgs   map[string]string
	LocalImage  string
	RemoteCache bool
}

type buildOptions struct {
	embeddedBuildOptions
}

func buildCmd() *cobra.Command {

	opts := &buildOptions{}

	buildCmd := &cobra.Command{
		Use:    "build path/to/module/dir",
		Short:  "uses Dockerfile to build an image for the module",
		Args:   cobra.MaximumNArgs(1),
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := cmdOpts.AuthProvider().Identify(cmd.Context())
			if err != nil {
				return errors.WithStack(err)
			}

			p, err := projectDir(args)
			if err != nil {
				return errors.WithStack(err)
			}
			jetCfg, err := loadOrInitConfigFromFileSystem(ctx, cmd, args)
			if err != nil {
				return errors.WithStack(err)
			}

			// Only needed because of --remote-cache:
			cluster, err := cmdOpts.ClusterProvider().Get(ctx)
			if err != nil {
				return errors.WithStack(err)
			}

			repoConfig, err := cmdOpts.RepositoryProvider().Get(ctx, cluster, "")
			if err != nil {
				return errors.WithStack(err)
			}

			pad := launchpad.NewPad(cmdOpts.ErrorLogger())
			jetlog.Logger(ctx).HeaderPrintf("Building project %s", jetCfg.GetProjectName())
			_, _, err = execLaunchpadBuild(ctx, pad, &opts.embeddedBuildOptions, jetCfg, cluster, p,
				repoConfig, "1/1" /*stepInfo */)
			if err != nil {
				return errors.WithStack(err)
			}
			jetlog.Logger(ctx).HeaderPrintf("App Built.\n")

			return nil
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			if err := cleanupPreviousBuildsPostRun(cmd, args); err != nil {
				cmdOpts.ErrorLogger().CaptureException(err)
				return
			}
		},
	}

	registerBuildFlags(buildCmd, opts)
	return buildCmd
}

// registerBuildFlags is only called by the build command. One call site.
func registerBuildFlags(cmd *cobra.Command, opts *buildOptions) {
	registerEmbeddedBuildFlags(cmd, &opts.embeddedBuildOptions)
	jflags.RegisterCommonFlags(cmd, cmdOpts)
}

// registerEmbeddedBuildFlags is called by many commands: up, dev, build, local.
func registerEmbeddedBuildFlags(cmd *cobra.Command, opts *embeddedBuildOptions) {

	cmd.Flags().StringVarP(
		&opts.Platform,
		"platform",
		"p",
		"linux/amd64", // sensible default for remote clusters
		"Platform architecture to build for. These are the same values "+
			"that docker respects. Examples: linux/amd64, linux/arm64.",
	)

	cmd.Flags().StringToStringVar(
		&opts.BuildArgs,
		"docker.build-arg",
		map[string]string{},
		"See docker --build-arg",
	)

	cmd.Flags().BoolVar(
		&opts.RemoteCache,
		"remote-cache",
		false,
		"[EXPERIMENTAL] Use buildkit remote cache to speed up builds. This flag "+
			"adds BUILDKIT_INLINE_CACHE=1 build arg to save cache metadata and also "+
			"sets the cache-from docker build option",
	)

	cmd.Flags().StringVar(
		&opts.LocalImage,
		localImageFlag,
		"",
		"If set, it uses the pre-built local-image instead of building a new one",
	)
	_ = cmd.Flags().MarkHidden(localImageFlag)
}

func makeBuildOptions(
	ctx context.Context,
	opts *embeddedBuildOptions,
	jetCfg *jetconfig.Config,
	cluster provider.Cluster,
	absPath string,
	repoConfig provider.RepoConfig,
) (*launchpad.BuildOptions, error) {
	suffix, err := kubevalidate.ToValidName(jetCfg.GetProjectNameWithSlug())
	if err != nil {
		return nil, err
	}
	imageRepoForCache := ""
	if repoConfig != nil {
		imageRepoForCache = repoConfig.GetImageRepoPrefix() + "/" + suffix
	}
	buildOpts := &launchpad.BuildOptions{
		AppName:           jetCfg.GetProjectName(),
		BuildArgs:         opts.BuildArgs,
		ImageRepoForCache: imageRepoForCache,
		LifecycleHook:     cmdOpts.Hooks().Build,
		LocalImage:        opts.LocalImage,
		ProjectDir:        absPath,
		ProjectId:         jetCfg.ProjectID,
		Platform:          opts.Platform,
		Services:          jetCfg.Builders(),
		RemoteCache:       opts.RemoteCache,
		RepoConfig:        repoConfig,
		TagPrefix:         cmdOpts.RootFlags().Env().ImageTagPrefix(),
	}

	return buildOpts, nil
}

func cleanupPreviousBuildsPostRun(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	absPath, err := projectDir(args)
	if err != nil {
		return errors.WithStack(err)
	}
	jetCfg, err := jetconfig.RequireFromFileSystem(ctx, absPath, cmdOpts.RootFlags().Env())
	if err != nil {
		return errors.WithStack(err)
	}
	// In the future we may have other cleanup tasks.
	// For now we are just cleaning up docker images.
	err = launchpad.DockerCleanup(ctx, jetCfg.ProjectID)
	return errors.WithStack(err)
}
