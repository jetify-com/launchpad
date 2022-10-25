package kubeconfig

// This file provides a "fluent-like" API that makes it easy to do method-chaining
// in order to initialize a client from a kubeconfig.

func FromDefaults() ClientBuilder {
	return NewClientBuilder()
}

func FromDefaultFile() ClientBuilder {
	return NewClientBuilder(
		WithLoader(&FilePathLoader{}),
	)
}

func FromFile(path string) ClientBuilder {
	return NewClientBuilder(
		WithLoader(FilePathLoader{
			Path: path,
		}),
	)
}

func FromYAML(yaml string) ClientBuilder {
	return NewClientBuilder(
		WithLoader(YamlLoader{
			YAML: yaml,
		}),
	)
}

func FromCurrentCluster() ClientBuilder {
	return NewClientBuilder(
		WithLoader(InClusterLoader{}),
	)
}
