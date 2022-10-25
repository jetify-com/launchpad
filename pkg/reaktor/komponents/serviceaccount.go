package komponents

import (
	"go.jetpack.io/launchpad/pkg/reaktor"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ServiceAccount implements interface Resource (compile-time check)
var _ reaktor.Resource = (*ServiceAccount)(nil)

type ServiceAccount struct {
	Name      string
	Namespace string
}

func (sa *ServiceAccount) ToManifest() (any, error) {
	manifest := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ServiceAccount",
			"metadata": map[string]any{
				"name":      sa.Name,
				"namespace": sa.Namespace,
			},
		},
	}
	return manifest, nil
}
