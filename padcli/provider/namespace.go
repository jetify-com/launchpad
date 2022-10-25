package provider

import (
	"context"

	"github.com/pkg/errors"
	"go.jetpack.io/launchpad/padcli/kubeconfig"
	"go.jetpack.io/launchpad/proto/api"
)

type NamespaceProvider interface {
	Get(ctx context.Context, ns string, kubeContext string, e api.Environment) (string, error)
}

func KubeConfigNamespaceProvider() NamespaceProvider {
	return &kubeConfigNamespaceProvider{}
}

type kubeConfigNamespaceProvider struct{}

var _ NamespaceProvider = (*kubeConfigNamespaceProvider)(nil)

func (p *kubeConfigNamespaceProvider) Get(
	ctx context.Context,
	ns string,
	kubeContextName string,
	e api.Environment,
) (string, error) {
	if ns != "" {
		return ns, nil
	}

	var err error
	if kubeContextName == "" {
		kubeContextName, err = kubeconfig.GetCurrentContextName()
		if err != nil {
			return "", errors.Wrap(err, "failed to get current kube context name")
		}
	}
	kubeContext, err := kubeconfig.GetContext(kubeContextName)
	if err != nil {
		return "", errors.Wrap(err, "failed to get kube context")
	}
	if kubeContext.Namespace != "" {
		return kubeContext.Namespace, nil
	} else {
		return "default", nil
	}
}
