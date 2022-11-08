package jflags

import (
	"github.com/spf13/cobra"
	"go.jetpack.io/launchpad/padcli/provider"
)

func RegisterCommonFlags(cmd *cobra.Command, p provider.Providers) {
	cmd.Flags().StringVarP(
		p.ClusterProvider().GetSelectedClusterName(),
		"cluster",
		"c",
		"",
		"The Kubernetes cluster to deploy to. Can be the name of Jetpack-managed cluster or the name of a context in your kubeconfig",
	)
}
