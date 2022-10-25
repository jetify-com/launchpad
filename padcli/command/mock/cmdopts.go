package mock

import (
	"context"

	"github.com/spf13/cobra"
	"go.jetpack.io/launchpad/padcli/flags"
	"go.jetpack.io/launchpad/padcli/hook"
	"go.jetpack.io/launchpad/padcli/provider"
	"go.jetpack.io/launchpad/proto/api"
)

// We should try to make all these mocks unnecessary. For example the Namespace
// Init method uses the auth provider to get user token, but that function should
// have a provider passed in when created. The default implementation of that provider
// should work with a nil user.

type MockCmdOptions struct {
	RootCMDFlags *flags.RootCmdFlags
}
type mockAnalyticsProvider struct{}
type mockAuthProvider struct{}
type mockClusterProvider struct{}
type mockNamespaceProvider struct{}
type mockUser struct{}

func (*MockCmdOptions) AdditionalCommands() []*cobra.Command {
	return nil
}

func (*MockCmdOptions) AnalyticsProvider() provider.Analytics {
	return &mockAnalyticsProvider{}
}

func (*MockCmdOptions) AuthProvider() provider.Auth {
	return &mockAuthProvider{}
}

func (*MockCmdOptions) ClusterProvider() provider.ClusterProvider {
	return &mockClusterProvider{}
}

func (*MockCmdOptions) EnvSecProvider() provider.EnvSec {
	return provider.DefaultEnvSecProvider()
}

func (*MockCmdOptions) ErrorLogger() provider.ErrorLogger {
	return &provider.NoOpLogger{}
}

func (*MockCmdOptions) Hooks() *hook.Hooks {
	return hook.New()
}

func (*MockCmdOptions) RepositoryProvider() provider.Repository {
	return provider.EmptyRepository()
}

func (*MockCmdOptions) NamespaceProvider() provider.NamespaceProvider {
	return &mockNamespaceProvider{}
}

func (m *MockCmdOptions) RootFlags() *flags.RootCmdFlags {
	return m.RootCMDFlags
}

func (m *MockCmdOptions) RootCommand() *cobra.Command {
	return &cobra.Command{}
}

func (*MockCmdOptions) PersistentPreRunE(cmd *cobra.Command, args []string) error {
	return nil
}

func (*MockCmdOptions) PersistentPostRunE(cmd *cobra.Command, args []string) error {
	return nil
}

func (*mockAnalyticsProvider) Track(ctx context.Context, event string, options map[string]interface{}) {
}

func (*mockAnalyticsProvider) Close() error {
	return nil
}

func (*mockAuthProvider) Identify(ctx context.Context) (context.Context, error) {
	return ctx, nil
}

func (*mockAuthProvider) User(ctx context.Context) (provider.User, error) {
	return &mockUser{}, nil
}

func (*mockUser) Token() string {
	return ""
}

func (*mockUser) OrgID() string {
	return ""
}

func (*mockUser) ID() string {
	return ""
}

func (*mockUser) Email() string {
	return ""
}

func (*mockClusterProvider) Get(
	ctx context.Context,
	kubeContextName string,
) (provider.Cluster, error) {
	return NewClusterForTest("local", true), nil
}

func (*mockClusterProvider) GetAll(
	ctx context.Context,
) ([]provider.Cluster, error) {
	return []provider.Cluster{
		NewClusterForTest("local", true),
		NewJetpackManagedClusterForTest("remote-jetpack", "jetpack-cluster"),
		NewJetpackManagedClusterForTest("remote-not-jetpack", "byoc-cluster"),
	}, nil
}

func (*mockNamespaceProvider) Get(
	ctx context.Context,
	ns string,
	kubeContextName string,
	e api.Environment,
) (string, error) {
	if ns != "" {
		return ns, nil
	}
	return "mock-namespace", nil
}

func NewClusterForTest(kubeContextName string, isLocal bool) provider.Cluster {
	isJetpackManaged := false
	return provider.KubeConfigCluster(
		"hostname",
		isJetpackManaged,
		kubeContextName,
		isLocal,
	)
}

func NewJetpackManagedClusterForTest(
	kubeContextName string,
	hostname string,
) provider.Cluster {
	isJetpackManaged := true
	isLocal := false
	return provider.KubeConfigCluster(
		hostname,
		isJetpackManaged,
		kubeContextName,
		isLocal,
	)
}
