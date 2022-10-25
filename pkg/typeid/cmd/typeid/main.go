package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.jetpack.io/launchpad/pkg/typeid"
)

var rootCmd = &cobra.Command{
	Use:   "typeid <type_prefix>",
	Short: "Generate a random typeid with the given type prefix",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		tid := typeid.New(args[0])
		fmt.Println(tid.String())
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func main() {
	Execute()
}
