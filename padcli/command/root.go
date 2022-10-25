package command

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

	surveyterminal "github.com/AlecAivazis/survey/v2/terminal"
	"github.com/fatih/color"
	"github.com/getsentry/sentry-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"go.jetpack.io/launchpad/goutil/errorutil"
	"go.jetpack.io/launchpad/padcli/flags"
	"go.jetpack.io/launchpad/padcli/hook"
	"go.jetpack.io/launchpad/padcli/provider"
	"go.jetpack.io/launchpad/padcli/terminal"
	"go.jetpack.io/launchpad/pkg/buildstamp"
	"go.jetpack.io/launchpad/pkg/jetlog"
	"golang.org/x/sys/unix"
)

// These options allow the CLI to be customized with additional commands and
// "providers" that can enhance functionality by using private services.
type cmdOptions interface {
	provider.Providers
	AdditionalCommands() []*cobra.Command
	RootCommand() *cobra.Command
	RootFlags() *flags.RootCmdFlags
	Hooks() *hook.Hooks
	PersistentPreRunE(cmd *cobra.Command, args []string) error
	PersistentPostRunE(cmd *cobra.Command, args []string) error
}

// This is global for now (for expediency). We could pass these options down
// to every function that needs them.
var cmdOpts cmdOptions

const environmentFlagName = "environment"

func registerRootCmdFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().BoolVarP(
		&cmdOpts.RootFlags().Debug,
		"debug",
		"d",
		false,
		"print debug output",
	)

	// We depend on this flag for the install.sh script. Before changing this
	// flag, make sure that auto-update script won't be broken.
	cmd.PersistentFlags().BoolVar(
		&cmdOpts.RootFlags().SkipVersionCheck,
		"skip-version-check",
		false,
		"Skip CLI version check",
	)
	_ = cmd.PersistentFlags().MarkHidden("skip-version-check")

	// to read this flag, one must use the cmdOpts.RootFlags().Env() function
	cmd.PersistentFlags().StringVar(
		&cmdOpts.RootFlags().Environment,
		environmentFlagName,
		"dev",
		"The name of the environment this command should operate on. One of: dev, prod",
	)
}

func NewRootCmd(opts cmdOptions) *cobra.Command {
	cmdOpts = opts
	rootCmd := &cobra.Command{
		Use:   "jetpack",
		Short: "Build scalable Kubernetes backends in minutes",
		Long:  "Build scalable Kubernetes backends in minutes",
		// If an error occurs then cobra will print the Usage (i.e. --help)
		// but we don't want that. This still prints usage if user types
		// --help, or `jetpack help <cmd>`.
		SilenceUsage: true,
		// We print the error via special handling in the Execute() function
		// so we silence it here. If this were false, then we would
		// double-print the error message.
		SilenceErrors:     true,
		PersistentPreRunE: persistentPreRunE,
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.WithStack(cmd.Help())
		},
		PersistentPostRunE: cmdOpts.PersistentPostRunE,
	}

	rootCmd.AddCommand(
		buildCmd(),
		configCmd(),
		devCmd(),
		downCmd(),
		initCmd(),
		localCmd(),
		envCmd(),
		upCmd(),
		updateCmd(),
		versionCmd(),
		genDocsCmd(),
	)

	rootCmd.AddCommand(cmdOpts.AdditionalCommands()...)

	registerRootCmdFlags(rootCmd)

	rootCmd.CompletionOptions.HiddenDefaultCmd = true

	return rootCmd
}

// Execute is the entry point for CLI app.
func Execute(ctx context.Context, opts cmdOptions) {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, unix.SIGTERM)

	span := sentry.StartSpan(ctx, "cliCommand")
	err := opts.RootCommand().ExecuteContext(ctx)
	span.Finish()

	if err != nil {
		// For now log all errors. If this gets too noisy, we can log only for stuff
		// that is not a user error.
		cmdOpts.ErrorLogger().CaptureException(err)
		if opts.RootFlags().Debug {
			logrus.SetLevel(logrus.DebugLevel)
			stackTrace := errorutil.EarliestStackTrace(err)
			errChainMsg := fmt.Sprintf("Error chain is:\n\t %s.\n\n", err.Error())
			if stackTrace != nil {
				log.Fatalf("%sStacktrace:\n%+v\n", errChainMsg, stackTrace)
			} else {
				log.Fatalf(
					"%sFailed to get Stacktrace:\n%+v\n",
					errChainMsg,
					errors.Cause(err),
				)
			}

		} else {
			displayed := cmdOpts.ErrorLogger().DisplayException(err)
			if displayed {
				// Error was displayed, but we still want to exit with non-zero code.
				os.Exit(1)
			}

			// user interrupt signals (ctrl+c) are not errors caused by user or jetpack
			// So they need special handling, clean output and graceful shutdown.
			if errors.Is(err, context.Canceled) || errors.Is(err, surveyterminal.InterruptErr) {
				fmt.Println("ABORT: Operation cancelled by user interruption.")
				stop()
				os.Exit(1)
			}

			// This logic allows us to handle errors, combined errors and user errors.
			// errors: normal golang errors
			// combined: golang error + user friendly error to display
			// user: no golang error cause, just a user error we created.
			if msg := errorutil.GetUserErrorMessage(err); msg != "" {
				color.Red(
					"\nError: %s\n\nCaused by:\n\n %s\n\nRun with --debug for more information",
					msg,
					err,
				)
				os.Exit(1)
			} else {
				log.Fatalf(
					"ABORT: There was an error. The cause is:\n\t %s. \n"+
						"Run with --debug for more information",
					errors.Cause(err),
				)
			}
		}
	}
}

// This function should never panic because it will
// cause auto update to be stuck in a forever loop.
func persistentPreRunE(cmd *cobra.Command, args []string) error {
	if err := cmdOpts.PersistentPreRunE(cmd, args); err != nil {
		return err
	}

	ctx := cmd.Context()
	// Auto-updating:
	autoUpdateBlacklist := map[string]bool{
		"jetpack help":                          true,
		"jetpack update":                        true,
		"jetpack version":                       true,
		"jetpack auth get-token":                true,
		"jetpack auth get-registry-credentials": true,
	}
	if terminal.IsInteractive() &&
		!cmdOpts.RootFlags().SkipVersionCheck &&
		!autoUpdateBlacklist[cmd.CommandPath()] {

		buildstmp := buildstamp.Get()
		if err := updateJetpack(ctx, cmd, args, buildstmp); err != nil {
			jetlog.Logger(ctx).Print(
				errors.Wrap(err, "ERROR: failed during auto-update").Error(),
			)
		}
	}

	if !cmdOpts.RootFlags().IsValidEnvironment() {
		return errorutil.NewUserErrorf(
			"Environment \"%s\" not recognized. Please use one of: dev, prod.\n",
			cmdOpts.RootFlags().Environment,
		)
	}

	projDir, err := projectDir(args)
	if err != nil {
		// if there is an error, then set projectDir to empty-string. See function
		// comment about why skipping error is advisable.
		projDir = ""
	}

	// deliberately ignore error (see reason above)
	_ = cmdOpts.Hooks().CommandStart(projDir)
	return nil
}
