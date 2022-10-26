package command

import (
	"context"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.jetpack.io/launchpad/launchpad"
	"go.jetpack.io/launchpad/pkg/jetlog"
)

type localOptions struct {
	sdkCmd              string
	execQualifiedSymbol string
	envOverrideFile     string
	embeddedBuildOptions
}

func localCmd() *cobra.Command {

	opts := &localOptions{}

	localCmd := &cobra.Command{
		Use:    "local path/to/module/dir",
		Short:  "uses Dockerfile to build an image and runs it locally",
		Args:   cobra.MaximumNArgs(1),
		Hidden: true,
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

			jetlog.Logger(ctx).Printf("Launchpad is preparing to build %s.\n", jetCfg.GetProjectName())

			// Only needed because of --remote-cache:
			cluster, err := cmdOpts.ClusterProvider().Get(ctx, opts.DefaultedCluster(jetCfg))
			if err != nil {
				return errors.WithStack(err)
			}

			buildOpts, err := makeBuildOptions(ctx, &opts.embeddedBuildOptions, jetCfg, cluster, absPath,
				nil /*repoConfig*/)
			if err != nil {
				return errors.WithStack(err)
			}

			pad := launchpad.NewPad(cmdOpts.ErrorLogger())
			bo, err := buildForRunningLocally(ctx, pad, buildOpts)
			if err != nil {
				return errors.Wrap(err, "failed to build")
			}
			localEnvVars := map[string]string{}
			// if --env-override flag was explicitly set, then populate localEnvVars
			if opts.envOverrideFile != "" {
				localEnvVars, err = readEnvVariables(
					buildOpts.ProjectDir,
					opts.envOverrideFile,
					false, // encodedValues
				)
				if err != nil {
					return errors.WithStack(err)
				}
			}

			store, err := newEnvStore(ctx, cmdOpts.EnvSecProvider())
			if err != nil {
				return errors.WithStack(err)
			}

			remoteEnvVars, err := getRemoteEnvVars(ctx, jetCfg, store)
			if err != nil {
				return errors.Wrap(err, "failed to retrieve env variables from launchpad env")
			}

			return errors.Wrap(
				pad.RunLocally(
					ctx,
					&launchpad.LocalOptions{
						BuildOut:            bo,
						SdkCmd:              opts.sdkCmd,
						ExecQualifiedSymbol: opts.execQualifiedSymbol,
						LocalEnvVars:        localEnvVars,
						RemoteEnvVars:       remoteEnvVars,
					},
				),
				"failed to run",
			)
		},
	}

	registerLocalCmdFlags(localCmd, opts)

	return localCmd
}

func buildForRunningLocally(
	ctx context.Context,
	pad *launchpad.Pad,
	buildOpts *launchpad.BuildOptions,
) (*launchpad.BuildOutput, error) {

	jetlog.Logger(ctx).HeaderPrintf("Building project %s locally", buildOpts.AppName)

	jetlog.Logger(ctx).HeaderPrintf("Step 1/1 Building Docker image...")
	buildOutput, err := pad.Build(ctx, buildOpts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build Docker image")
	}
	jetlog.Logger(ctx).HeaderPrintf("[DONE] Successfully built Dockerfile\n")
	jetlog.Logger(ctx).HeaderPrintf("App Built.\n")
	return buildOutput, nil
}

func registerLocalCmdFlags(cmd *cobra.Command, opts *localOptions) {

	cmd.Flags().StringVar(
		&opts.sdkCmd,
		"sdk-cmd",
		"",
		"commands to invoke on the jetpack SDK within the container",
	)

	cmd.Flags().StringVar(
		&opts.execQualifiedSymbol,
		"exec",
		"",
		"Execute a single cronjob or jetroutine for a given function.",
	)

	cmd.Flags().StringVar(
		&opts.envOverrideFile,
		envOverrideFlag,
		"",
		envOverrideFlagMsg,
	)

	registerEmbeddedBuildFlags(cmd, &opts.embeddedBuildOptions)

	// This overrides the default since in local mode we want to default to system
	// platform.
	cmd.Flags().Lookup("platform").DefValue = ""
}
