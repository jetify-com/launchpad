package command

import (
	"context"
	"os"
	"path/filepath"

	"github.com/MakeNowJust/heredoc"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.jetpack.io/envsec"
	"go.jetpack.io/envsec/envcli"
	"go.jetpack.io/launchpad/padcli/jetconfig"
	"go.jetpack.io/launchpad/padcli/provider"
	"go.jetpack.io/launchpad/pkg/jetlog"
)

type envOptions struct {
	projectConfigPath string
}

func envCmd() *cobra.Command {
	opts := &envOptions{}
	cmdCfg := &envcli.CmdConfig{}
	command := &cobra.Command{
		Use:   "env",
		Short: "Manage environment variables and secrets",
		Long: heredoc.Doc(`
			Manage environment variables and secrets

			Securely stores and retrieves environment variables on the cloud.
			Environment variables are always encrypted, which makes it possible to
			store values that contain passwords and other secrets.
		`),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			var absProjectPath string
			var err error
			if opts.projectConfigPath == "" {
				absProjectPath, err = getProjectDir()
			} else {
				absProjectPath, err = absPath([]string{opts.projectConfigPath})
			}
			if err != nil {
				return errors.WithStack(err)
			}

			jetCfg, err := RequireConfigFromFileSystem(cmd.Context(), cmd, []string{absProjectPath}, cmdOpts)
			if err != nil {
				return errors.WithStack(err)
			}

			ctx, err := cmdOpts.AuthProvider().Identify(cmd.Context())
			if err != nil {
				return errors.WithStack(err)
			}

			// Construct the envId:
			envId, err := cmdOpts.EnvSecProvider().NewEnvId(
				ctx,
				jetCfg.GetProjectID(),
				cmdOpts.RootFlags().Env().String(),
			)
			if err != nil {
				return errors.WithStack(err)
			}
			if envId == nil {
				return errors.New("unexpected nil envId")
			}
			cmdCfg.EnvId = *envId

			store, err := newEnvStore(ctx, cmd, args, cmdOpts.EnvSecProvider(), jetCfg.Envsec.Provider)
			if err != nil {
				return errors.WithStack(err)
			}
			cmdCfg.Store = store
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.WithStack(cmd.Help())
		},
	}

	command.PersistentFlags().StringVarP(
		&opts.projectConfigPath,
		"project",
		"p",
		"",
		"Path to project config. If directory, we assume name is launchpad.yaml",
	)

	command.AddCommand(
		envcli.SetCmd(cmdCfg),
		envcli.RemoveCmd(cmdCfg),
		envcli.ListCmd(cmdCfg),
		envcli.UploadCmd(cmdCfg),
		envcli.DownloadCmd(cmdCfg),
		envcli.ExecCmd(cmdCfg),
	)
	return command
}

func getProjectDir() (string, error) {
	// get absolute path of current working directory
	currentPath, err := filepath.Abs(".")
	if err != nil {
		return "", errors.WithStack(err)
	}
	// look up launchpad.yaml in all parents of working directory
	// until the working directory is root
	configFileName := jetconfig.ConfigName(currentPath)
	for {
		_, err = os.Stat(filepath.Join(currentPath, configFileName))
		if err == nil {
			// config file found
			return currentPath, nil
		} else if !os.IsNotExist(err) {
			return "", errors.WithStack(err)
		}
		if currentPath == "/" {
			break
		}
		currentPath = filepath.Dir(currentPath)
	}
	// Ignoring the os.Stat error since it will be confusing to the user seeing "Stat /home/launchpad.yaml no such file"
	return "", errors.New(
		"'launchpad env' only works within a Launchpad project's directory. Please change your current directory to a Launchpad project and try again",
	)
}

func newEnvStore(
	ctx context.Context,
	cmd *cobra.Command,
	args []string,
	envSecProvider provider.EnvSec,
	selectedProvider string,
) (envsec.Store, error) {
	storeConfig := &envsec.SSMConfig{}

	providedConfig, err := envSecProvider.Get(ctx, selectedProvider)
	if err != nil {
		return nil, err
	}
	if providedConfig == nil && selectedProvider == "" {
		// Skip envsec as the project is not setup with envsec.
		return nil, nil
	}

	if providedConfig != nil {
		storeConfig = &envsec.SSMConfig{
			Region:          providedConfig.GetRegion(),
			AccessKeyId:     providedConfig.GetAccessKeyId(),
			SecretAccessKey: providedConfig.GetSecretAccessKey(),
			SessionToken:    providedConfig.GetSessionToken(),
			KmsKeyId:        providedConfig.GetKmsKeyId(),
		}
	}

	store, err := envsec.NewStore(ctx, storeConfig)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	// Set jetconfig Envsec field.
	path, err := absPath(args)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	jetCfg, err := jetconfig.RequireFromFileSystem(ctx, path, cmdOpts.RootFlags().Env())
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if jetCfg.Envsec.Provider != jetconfig.JetpackEnvsecProvider {
		jetCfg.Envsec.Provider = jetconfig.JetpackEnvsecProvider
		_, err = jetCfg.SaveConfig(path)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		jetlog.Logger(ctx).Println("We have updated your project's launchpad.yaml. Please commit that to your repository.")
	}

	return store, nil
}
