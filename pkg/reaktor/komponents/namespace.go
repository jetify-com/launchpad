package komponents

import (
	"go.jetpack.io/launchpad/pkg/reaktor"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Namespace struct {
	Name   string
	Labels map[string]string
}

// Namespace implements interface Resource (compile-time check)
var _ reaktor.Resource = (*Namespace)(nil)

func (ns *Namespace) ToManifest() (any, error) {
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Namespace",
			"metadata": map[string]any{
				"name":   ns.Name,
				"labels": ns.Labels,
			},
		},
	}, nil
}
