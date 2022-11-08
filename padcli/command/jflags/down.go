package jflags

import (
	"github.com/spf13/cobra"
	"go.jetpack.io/launchpad/padcli/provider"
)

func NewDownCmd() *DownCmd {
	return &DownCmd{}
}

func RegisterDownFlags(cmd *cobra.Command, flags *DownCmd, p provider.Providers) {
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
	RegisterCommonFlags(cmd, p)
}

type DownCmd struct {
	app       string
	namespace string
}

func (f *DownCmd) App() string {
	return f.app
}

func (f *DownCmd) Namespace() string {
	return f.namespace
}
