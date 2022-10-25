package jflags

import (
	"github.com/spf13/cobra"
)

func NewDownCmd() *DownCmd {
	return &DownCmd{}
}

func RegisterDownFlags(cmd *cobra.Command, flags *DownCmd) {
	cmd.Flags().StringVarP(
		&flags.namespace,
		"namespace",
		"n",
		"",
		"K8s Namespace",
	)
	cmd.Flags().StringVar(
		&flags.app,
		"helm.app.name",
		"",
		"App install name",
	)
	RegisterCommonFlags(cmd, &flags.Common)
}

type DownCmd struct {
	Common

	app       string
	namespace string
}

func (f *DownCmd) App() string {
	return f.app
}

func (f *DownCmd) Namespace() string {
	return f.namespace
}
