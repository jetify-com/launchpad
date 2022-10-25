package komponents

import (
	"go.jetpack.io/launchpad/pkg/reaktor"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Pod struct {
	Labels    map[string]any
	Namespace string
	Spec      map[string]any
}

// Pod implements interface Resource (compile-time check)
var _ reaktor.Resource = (*Pod)(nil)

func (p *Pod) ToManifest() (any, error) {
	// This implementation is incomplete. Added it as a helper for podSpec
	// marshalling. It can be fleshed out as needed.
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]any{
				"namespace": p.Namespace,
				"labels":    p.Labels,
			},
			"spec": p.Spec,
		},
	}, nil
}
