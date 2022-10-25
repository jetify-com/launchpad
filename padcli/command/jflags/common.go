package jflags

import (
	"github.com/spf13/cobra"
	"go.jetpack.io/launchpad/goutil"
)

func RegisterCommonFlags(cmd *cobra.Command, flags *Common) {
	cmd.Flags().StringVarP(
		&flags.cluster,
		"cluster",
		"c",
		"",
		"The Kubernetes cluster to deploy to. Can be the name of Jetpack-managed cluster or the name of a context in your kubeconfig",
	)
}

type Common struct {
	cluster string
}

type clusterProvider interface {
	GetCluster() string
}

func (c *Common) DefaultedCluster(p clusterProvider) string {
	return goutil.Coalesce(c.cluster, p.GetCluster())
}
