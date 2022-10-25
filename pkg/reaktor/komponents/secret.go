package komponents

import (
	"go.jetpack.io/launchpad/pkg/reaktor"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Secret struct {
	Name      string
	Namespace string
	Type      string
	Data      map[string]any
}

// Secret implements interface Resource (compile-time check)
var _ reaktor.Resource = (*Secret)(nil)

// ToManifest is only structured for reading. For creating secrets more work
// needs to be done.
func (ns *Secret) ToManifest() (any, error) {
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]any{
				"name":      ns.Name,
				"namespace": ns.Namespace,
			},
			"type": ns.Type,
			"data": ns.Data,
		},
	}, nil
}

func SecretFromUnstructured(u *unstructured.Unstructured) *Secret {
	return &Secret{
		Name:      u.GetName(),
		Namespace: u.GetNamespace(),
		Type:      u.Object["type"].(string),
		Data:      u.Object["data"].(map[string]any),
	}
}
