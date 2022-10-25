package provider

// Keep abc
type Providers interface {
	AnalyticsProvider() Analytics
	AuthProvider() Auth
	ClusterProvider() ClusterProvider
	EnvSecProvider() EnvSec
	ErrorLogger() ErrorLogger
	NamespaceProvider() NamespaceProvider
	RepositoryProvider() Repository
}
