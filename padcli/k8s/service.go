package k8s

import (
	"context"

	"github.com/pkg/errors"
	"go.jetpack.io/launchpad/pkg/reaktor"
	"go.jetpack.io/launchpad/pkg/reaktor/kubeconfig"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ServiceNodePort(
	ctx context.Context,
	name, ns, kubeCtx string,
) (int, error) {
	klient, err := reaktor.WithClientBuilder(
		kubeconfig.NewClientBuilder(kubeconfig.WithFlags(&kubeconfig.Flags{
			Context: kubeCtx,
		})),
	)
	if err != nil {
		return 0, errors.WithStack(err)
	}

	serviceData, err := klient.Get(
		ctx,
		reaktor.ServiceGVR(),
		name,
		ns,
	)
	if err != nil {
		return 0, errors.WithStack(err)
	}

	ports, found, err := unstructured.NestedSlice(
		serviceData.Object,
		"spec",
		"ports",
	)
	if err != nil {
		return 0, errors.WithStack(err)
	} else if !found {
		return 0, errors.Errorf("service %s/%s has no ports", ns, name)
	}

	return int(ports[0].(map[string]any)["nodePort"].(int64)), nil
}
