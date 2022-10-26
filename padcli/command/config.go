package command

import (
	"context"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.jetpack.io/launchpad/padcli/jetconfig"
	"go.jetpack.io/launchpad/pkg/jetlog"
)

var UnrecognizedCommandFlagError = errors.New("unrecognized command flag")

func configCmd() *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "commands to operate on a project's jetconfig",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	upgradeCmd := &cobra.Command{
		Use:   "upgrade [path]",
		Short: "upgrades a project's launchpad.yaml to follow the latest schema",
		Long: "upgrades a project's launchpad.yaml to follow the latest schema found " +
			"at https://www.jetpack.io/launchpad/docs/reference/launchpad.yaml-reference/",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := cmdOpts.AuthProvider().Identify(cmd.Context())
			if err != nil {
				return errors.WithStack(err)
			}

			jetlog.Logger(ctx).HeaderPrintf("Step 1/1 checking if the jetconfig needs to upgrade.\n")

			// This calls upgrade internally.
			_, err = RequireConfigFromFileSystem(ctx, cmd, args)
			if err != nil {
				return errors.WithStack(err)
			}

			jetlog.Logger(ctx).HeaderPrintf("[DONE] Finished ensuring jetconfig is the latest version.\n")
			return nil
		},
	}
	configCmd.AddCommand(upgradeCmd)

	return configCmd
}

func RequireConfigFromFileSystem(
	ctx context.Context,
	cmd *cobra.Command,
	cmdArgs []string,
) (*jetconfig.Config, error) {
	p, err := absPath(cmdArgs)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	c, err := jetconfig.RequireFromFileSystem(ctx, p, cmdOpts.RootFlags().Env())
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if err := addFlagsToCmd(cmd, cmdArgs, c); err != nil {
		return nil, errors.WithStack(err)
	}
	if !c.HasDefaultFileName() {
		jetlog.Logger(ctx).Printf("Using config at %s\n\n", c.Path)
	}
	return c, nil
}

// Loads the launchpad.yaml or jetconfig.yaml file
// If file does not exist, drop users back into the launchpad init flow
func loadOrInitConfigFromFileSystem(
	ctx context.Context,
	cmd *cobra.Command,
	cmdArgs []string,
) (*jetconfig.Config, error) {
	c, err := RequireConfigFromFileSystem(ctx, cmd, cmdArgs)
	if !errors.Is(err, jetconfig.ErrConfigNotFound) {
		return c, errors.WithStack(err)
	}

	p, err := absPath(cmdArgs)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	configFileName := jetconfig.ConfigName(p)
	jetlog.Logger(ctx).Printf("Cannot find %s at path %s. "+
		"Running `launchpad init <app-directory>`\n", configFileName, p)
	jetlog.Logger(ctx).Println(
		"Setting up your launchpad application. Press `ctrl-c` to exit")
	if err := initConfig(ctx, p); err != nil {
		return nil, errors.WithStack(err)
	}
	c, err = RequireConfigFromFileSystem(ctx, cmd, cmdArgs)
	return c, errors.WithStack(err)
}

func addFlagsToCmd(cmd *cobra.Command, cmdArgs []string, cfg *jetconfig.Config) error {
	cfgEnvironment, ok := cfg.Environment[strings.ToLower(cmdOpts.RootFlags().Env().String())]
	if !ok {
		// If we didn't find any environment, then we can silently exit.
		// The environment field is optional.
		return nil
	}
	cfgFlags := cfgEnvironment.Flags
	p, err := absPath(cmdArgs)
	if err != nil {
		return errors.WithStack(err)
	}
	configFileName := jetconfig.ConfigName(p)

	for flagName := range cfgFlags {
		// Ignore flags from the config that don't apply to this command
		// or have been explicitly set via the command line.
		flag := cmd.Flags().Lookup(flagName)
		if flag == nil || flag.Changed {
			continue
		}

		// Setting the flag value
		if slice, ok := flag.Value.(pflag.SliceValue); ok {
			values, err := cfgFlags.GetValueAsStringSlice(flagName)
			if err != nil {
				return errors.Wrapf(
					err,
					"failed to read flag %s values from %s",
					flagName,
					configFileName,
				)
			}
			err = slice.Replace(values)
			if err != nil {
				return errors.Wrapf(err,
					"failed to set flag %s values %s as specified in %s",
					flagName,
					values,
					configFileName,
				)
			}
		} else {
			value, err := cfgFlags.GetValueAsString(flagName)
			if err != nil {
				return errors.Wrapf(err,
					"failed to read flag %s values from %s",
					flagName,
					configFileName,
				)
			}
			err = flag.Value.Set(value)
			if err != nil {
				return errors.Wrapf(err,
					"failed to set flag %s value %s as specified in %s",
					flagName,
					value,
					configFileName,
				)
			}
		}
		flag.Changed = true
	}

	return nil
}
