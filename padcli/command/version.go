package command

import (
	"os"

	"github.com/spf13/cobra"
	"go.jetpack.io/launchpad/pkg/buildstamp"
	"go.jetpack.io/launchpad/pkg/jetlog"
)

const binaryName = "launchpad"

func versionCmd() *cobra.Command {
	verboseFlag := false
	shortFlag := false

	var versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Prints the version number",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			buildstmp := buildstamp.Get()
			v := buildstmp.Version()
			if shortFlag {
				jetlog.Logger(ctx).Println(v)
				return nil
			}
			jetlog.Logger(ctx).Printf("%v %v\n", binaryName, v)
			if verboseFlag {
				buildstamp.PrintVerboseVersion(os.Stdout)
			}
			return nil
		},
	}
	versionCmd.Flags().BoolVarP(
		&verboseFlag,
		"verbose",
		"v",
		false, // value
		"Set to true for verbose output",
	)
	versionCmd.Flags().BoolVarP(
		&shortFlag,
		"short",
		"s",
		false, // value
		"Set to true for short output",
	)
	return versionCmd
}
