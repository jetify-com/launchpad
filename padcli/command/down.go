package command

import (
	"context"
	"fmt"

	"github.com/fatih/color"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.jetpack.io/launchpad/goutil/errorutil"
	"go.jetpack.io/launchpad/launchpad"
	"go.jetpack.io/launchpad/padcli/command/jflags"
	"go.jetpack.io/launchpad/padcli/jetconfig"
	"go.jetpack.io/launchpad/pkg/jetlog"
)

func downCmd() *cobra.Command {

	flags := jflags.NewDownCmd()

	downCmd := &cobra.Command{
		Use:   "down",
		Short: "Uninstalls the app",
		Long: "Uninstalls the app. Uses helm uninstall to remove the app and " +
			"associated resources.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := RequireConfigFromFileSystem(cmd.Context(), cmd, args, cmdOpts)
			if errors.Is(err, jetconfig.ErrConfigNotFound) {
				return errorutil.NewUserError(
					"jetconfig not found. Please run `launchpad down` in launchpad project " +
						"directory or pass path to directory as parameter.",
				)
			} else if err != nil {
				return errors.WithStack(err)
			}
			ctx, err := cmdOpts.AuthProvider().Identify(cmd.Context())
			if err != nil {
				return errors.WithStack(err)
			}
			do, err := makeLaunchpadDownOptions(ctx, c, flags)
			if err != nil {
				return errors.WithStack(err)
			}

			l := jetlog.Logger(ctx)
			boldSprint := color.New(color.Bold).Sprint
			l.HeaderPrintf("Uninstalling project %s", c.GetProjectName())
			fmt.Fprintln(l)
			fmt.Fprintln(l, "\tNamespace: "+boldSprint(do.Namespace))
			fmt.Fprintln(l, "\tCluster:   "+boldSprint(do.KubeContext))
			fmt.Fprintln(l)
			l.HeaderPrintf("Step 1/1 bringing down App and Launchpad resources...")

			err = launchpad.NewPad(cmdOpts.ErrorLogger()).Down(ctx, do)

			if err == nil {
				l.HeaderPrintf("[DONE] Successfully uninstalled %s.\n", do.InstanceName)
			} else {
				l.HeaderPrintf("[ERROR] Failed to uninstall %s successfully\n", do.InstanceName)
			}

			return errors.WithStack(err)
		},
	}

	jflags.RegisterDownFlags(downCmd, flags, cmdOpts)
	return downCmd
}

func makeLaunchpadDownOptions(
	ctx context.Context,
	jetCfg *jetconfig.Config,
	flags *jflags.DownCmd,
) (*launchpad.DownOptions, error) {
	cluster, err := cmdOpts.ClusterProvider().Get(ctx)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	ns, err := cmdOpts.NamespaceProvider().Get(ctx, flags.Namespace(), cluster.GetKubeContext(), cmdOpts.RootFlags().Env())
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &launchpad.DownOptions{
		ExternalCharts: jetconfigHelmToChartConfig(jetCfg, ns),
		ReleaseName:    getReleaseName(jetCfg),
		InstanceName:   getInstanceName(jetCfg),
		Namespace:      ns,
		KubeContext:    cluster.GetKubeContext(),
	}, nil
}
