package reaktor

import (
	"github.com/pkg/errors"
	"go.jetpack.io/launchpad/pkg/reaktor/kubeconfig"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

type Config struct {
	// Provide either a path to a kubeconfig or the yaml contents. Both can be
	// left empty and we'll search for the kubeconfig in the default places
	// (including ~/.kube and in-cluster)
	KubeConfigPath string
	KubeConfigYAML string
}

type Option func(*Config)

func New(opts ...Option) (*Reaktor, error) {
	// Defaults:
	cfg := &Config{}

	// Apply options:
	for _, opt := range opts {
		opt(cfg)
	}

	c, err := NewWithConfig(cfg)
	return c, errors.WithStack(err)
}

func WithYAML(yaml string) Option {
	return func(c *Config) {
		c.KubeConfigYAML = yaml
	}
}

func WithFile(kubeConfigPath string) Option {
	return func(c *Config) {
		c.KubeConfigPath = kubeConfigPath
	}
}

func NewWithConfig(cfg *Config) (*Reaktor, error) {
	err := validateConfig(cfg)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	// Should this logic move down to the kubeconfig client builder?
	var builder kubeconfig.ClientBuilder
	if cfg.KubeConfigPath == "" && cfg.KubeConfigYAML == "" {
		builder = kubeconfig.FromDefaults()
	} else if cfg.KubeConfigPath != "" {
		builder = kubeconfig.FromFile(cfg.KubeConfigPath)
	} else if cfg.KubeConfigYAML != "" {
		builder = kubeconfig.FromYAML(cfg.KubeConfigYAML)
	} else {
		return nil, errors.Errorf("Invalid configuration: %v", cfg)
	}

	if err != nil {
		return nil, errors.WithStack(err)
	}

	klient, err := WithClientBuilder(builder)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return klient, nil
}

func validateConfig(cfg *Config) error {
	if cfg.KubeConfigPath != "" && cfg.KubeConfigYAML != "" {
		return errors.New("reaktor: provide either a kubeconfig path or the yaml contents, but not both")
	}
	return nil
}

func WithClientBuilder(builder kubeconfig.ClientBuilder) (*Reaktor, error) {
	dynamicClient, err := builder.ToDynamicClient()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	mapper, err := builder.ToRESTMapper()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	klient := &Reaktor{
		dynamicClient: dynamicClient,
		factory:       cmdutil.NewFactory(builder),
		mapper:        mapper,
	}
	return klient, nil
}
