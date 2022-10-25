package kubeconfig

import "k8s.io/client-go/tools/clientcmd"

// Loader loads a kubeconfig and returns the corresponding ClientConfig object.
// Different implementations load from different sources.
type Loader interface {
	clientConfig(overrides *clientcmd.ConfigOverrides) (clientcmd.ClientConfig, error)
}

// DefaultLoader loads the kubeconfig from the current cluster if its running
// inside of a cluster. If its not running in a cluster it looks for a kubeconfig
// in the default file paths following the same logic as FilePathLoader.
type DefaultLoader struct{}

var _ Loader = (*DefaultLoader)(nil)

func (l DefaultLoader) clientConfig(overrides *clientcmd.ConfigOverrides) (clientcmd.ClientConfig, error) {
	// Standard precedence is to try kubeconfig first, then cluster. See for example,
	// what skaffold does: https://github.com/GoogleContainerTools/skaffold/blob/311b521c088bc39d412c28b6234b6077ca9054e7/pkg/skaffold/kubernetes/context/context.go#L98
	//
	// We're choosing to reverse that. Is that ok?
	var loader Loader
	if InClusterPossible() {
		loader = InClusterLoader{}
	} else {
		loader = FilePathLoader{}
	}
	return loader.clientConfig(overrides)
}

// FilePathLoader loads kubeconfig files from the filesystem. It searches for
// a kubeconfig file in the following order:
// + If `Path` is set, it looks for a kubeconfig at that path.
// + The list of paths specified in the `$KUBECONFIG` env variable.
// + ${HOME}/.kube/config
type FilePathLoader struct {
	Path string // Path to the kubeconfig file to use. Leave empty to use default paths
}

var _ Loader = (*FilePathLoader)(nil)

func (l FilePathLoader) clientConfig(overrides *clientcmd.ConfigOverrides) (clientcmd.ClientConfig, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	// use the standard defaults for this client command
	// DEPRECATED: remove and replace with something more accurate once available
	loadingRules.DefaultClientConfig = &clientcmd.DefaultClientConfig

	if l.Path != "" {
		loadingRules.ExplicitPath = l.Path
	}

	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides), nil
}

// InClusterLoader loads the kubeconfig file from the service account in the
// current cluster. The program must be running inside a k8s cluster for this to
// work.
type InClusterLoader struct{}

var _ Loader = (*InClusterLoader)(nil)

func (l InClusterLoader) clientConfig(overrides *clientcmd.ConfigOverrides) (clientcmd.ClientConfig, error) {
	return NewInClusterClientConfig(overrides), nil
}

// YamlLoader uses the provided kubeconfig YAML contents and does *not* rely on
// a file.
type YamlLoader struct {
	YAML string // YAML *contents* from a kubeconfig file
}

var _ Loader = (*YamlLoader)(nil)

func (l YamlLoader) clientConfig(overrides *clientcmd.ConfigOverrides) (clientcmd.ClientConfig, error) {
	config, err := clientcmd.Load([]byte(l.YAML))
	if err != nil {
		return nil, err
	}

	return clientcmd.NewNonInteractiveClientConfig(*config, "", overrides, nil /* configAccess */), nil
}
