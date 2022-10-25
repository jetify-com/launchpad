package komponents

import (
	"go.jetpack.io/launchpad/pkg/reaktor"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ConfigMap struct {
	Name      string
	Namespace string
}

// ConfigMap implements interface Resource (compile-time check)
var _ reaktor.Resource = (*ConfigMap)(nil)

// ToManifest is only structured for reading. For creating configMaps more work
// needs to be done.
func (ns *ConfigMap) ToManifest() (any, error) {
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":      ns.Name,
				"namespace": ns.Namespace,
			},
		},
	}, nil
}
