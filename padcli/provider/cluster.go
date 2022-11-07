package provider

import (
	"context"
	"strings"

	"github.com/pkg/errors"
	"go.jetpack.io/launchpad/padcli/kubeconfig"
)

type Cluster interface {
	GetHostname() string
	GetKubeContext() string

	IsJetpackManaged() bool
	IsLocal() bool
	IsRemoteUnmanaged() bool

	GetIsPrivate() bool

	GetName() string
}

type ClusterProvider interface {
	// Get returns the preferred Cluster to be used for the current command.
	Get(ctx context.Context, kubeContextName string) (Cluster, error)

	// GetAll returns all Clusters available to the user. May be empty.
	GetAll(ctx context.Context) ([]Cluster, error)
}

type kubeConfigCluster struct {
	hostname        string // always empty except for testing
	jetpackManaged  bool   // always false except for testing. TODO(DEV-1186)
	kubeContextName string
	local           bool
}

func KubeConfigCluster(
	hostname string,
	jetpackManaged bool,
	kubeContextName string,
	local bool,
) Cluster {
	return &kubeConfigCluster{
		hostname:        hostname,
		jetpackManaged:  jetpackManaged,
		kubeContextName: kubeContextName,
		local:           local,
	}
}

func (c *kubeConfigCluster) GetHostname() string {
	return c.hostname
}

func (c *kubeConfigCluster) GetKubeContext() string {
	return c.kubeContextName
}

func (c *kubeConfigCluster) IsJetpackManaged() bool {
	return c.jetpackManaged
}

func (c *kubeConfigCluster) IsLocal() bool {
	return c.local
}

func (c *kubeConfigCluster) IsRemoteUnmanaged() bool {
	return !c.local && !c.jetpackManaged
}

func (c *kubeConfigCluster) GetIsPrivate() bool {
	return true
}

func (c *kubeConfigCluster) GetName() string {
	return c.GetKubeContext()
}

type kubeConfigClusterProvider struct{}

var _ ClusterProvider = (*kubeConfigClusterProvider)(nil)

func KubeConfigClusterProvider() ClusterProvider {
	return &kubeConfigClusterProvider{}
}

func (p *kubeConfigClusterProvider) Get(ctx context.Context, kubeContextName string) (Cluster, error) {
	return toKubeConfigCluster(kubeContextName)
}

func (p *kubeConfigClusterProvider) GetAll(ctx context.Context) ([]Cluster, error) {
	names, err := kubeconfig.GetContextNames()
	if err != nil {
		return nil, err
	}

	clusters := []Cluster{}
	for _, n := range names {
		c, err := toKubeConfigCluster(n)
		if err != nil {
			return nil, err
		}
		clusters = append(clusters, c)
	}

	return clusters, nil
}

func IsLocalCluster(kubeContext string) (bool, error) {
	server, err := kubeconfig.GetServer(kubeContext)
	if err != nil {
		return false, errors.Wrap(err, "failed to get server from kube context")
	}

	const dockerDesktop = "https://kubernetes.docker.internal:6443"
	return server == dockerDesktop, nil
}

func isJetpackManagedCluster(kubeContext string) (bool, error) {
	ctx, err := kubeconfig.GetContext(kubeContext)
	if err != nil {
		return false, err
	}
	authInfo, err := kubeconfig.GetAuthInfo(ctx.AuthInfo)
	if err != nil {
		return false, err
	}

	// NOTE: this is a hacky way to determine whether a cluster in a kubeconfig is managed
	// by Jetpack. No other cluster would be using `launchpad` as an auth provider.
	return authInfo.Exec != nil && strings.HasPrefix(authInfo.Exec.Command, "launchpad"), nil
}

func toKubeConfigCluster(kubeContextName string) (Cluster, error) {
	isLocal, err := IsLocalCluster(kubeContextName)
	if err != nil {
		return nil, err
	}

	isJetpackManaged, err := isJetpackManagedCluster(kubeContextName)
	if err != nil {
		return nil, err
	}

	if isJetpackManaged && isLocal {
		return nil, errors.New("invalid cluster read from kubeconfig; a cluster cannot be local and jetpack-managed at the same time")
	}

	return &kubeConfigCluster{
		kubeContextName: kubeContextName,
		local:           isLocal,
		jetpackManaged:  isJetpackManaged,
	}, nil
}
