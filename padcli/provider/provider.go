package provider

// Keep abc
type Providers interface {
	AnalyticsProvider() Analytics
	AuthProvider() Auth
	ClusterProvider() ClusterProvider
	EnvSecProvider() EnvSec
	ErrorLogger() ErrorLogger
	InitSurveyProvider() InitSurveyProvider
	NamespaceProvider() NamespaceProvider
	RepositoryProvider() Repository
}
