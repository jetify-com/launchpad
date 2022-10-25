package komponents

import (
	"go.jetpack.io/launchpad/pkg/reaktor"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// NetworkPolicy is a simplified version of k8s NetworkPolicy resource
// For now, we only support limiting ingress based on namespaces. We can
// make this more complicated if needed
type NetworkPolicy struct {
	Name              string
	Namespace         string
	AllowedNamespaces []string
}

// NetworkPolicy implements interface Resource (compile-time check)
var _ reaktor.Resource = (*NetworkPolicy)(nil)

// ToManifest is only structured for reading. For creating secrets more work
// needs to be done.
func (np *NetworkPolicy) ToManifest() (any, error) {
	from := []map[string]any{}
	for _, ns := range np.AllowedNamespaces {
		from = append(from, map[string]any{
			"namespaceSelector": map[string]any{
				"matchLabels": map[string]any{
					"kubernetes.io/metadata.name": ns,
				},
			},
		})
	}

	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "networking.k8s.io/v1",
			"kind":       "NetworkPolicy",
			"metadata": map[string]any{
				"name":      np.Name,
				"namespace": np.Namespace,
			},
			"spec": map[string]any{
				"podSelector": map[string]any{},
				"ingress": []map[string]any{
					{
						"from": from,
					},
				},
			},
		},
	}, nil
}
