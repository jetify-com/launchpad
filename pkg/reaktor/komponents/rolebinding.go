package komponents

import (
	"go.jetpack.io/launchpad/pkg/reaktor"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// RoleBinding implements interface Resource (compile-time check)
var _ reaktor.Resource = (*RoleBinding)(nil)

type Subject struct {
	Kind      string
	Name      string
	Namespace string
}

type RoleRef struct {
	Kind string
	Name string
}

type RoleBinding struct {
	Name      string
	Namespace string
	RoleRef   *RoleRef
	Subjects  []*Subject
}

func (rb *RoleBinding) ToManifest() (any, error) {
	var subjects []map[string]any
	for _, s := range rb.Subjects {
		subjects = append(subjects, map[string]any{
			"kind":      s.Kind,
			"name":      s.Name,
			"namespace": s.Namespace,
		})
	}
	manifest := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "rbac.authorization.k8s.io/v1",
			"kind":       "RoleBinding",
			"metadata": map[string]any{
				"name":      rb.Name,
				"namespace": rb.Namespace,
			},
			"subjects": subjects,
			"roleRef": map[string]any{
				"apiGroup": "rbac.authorization.k8s.io",
				"kind":     rb.RoleRef.Kind,
				"name":     rb.RoleRef.Name,
			},
		},
	}
	return manifest, nil
}
