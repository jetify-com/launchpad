package komponents

import (
	"go.jetpack.io/launchpad/pkg/reaktor"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ClusterRoleBinding implements interface Resource (compile-time check)
var _ reaktor.Resource = (*ClusterRoleBinding)(nil)

type ClusterRoleBinding struct {
	Name        string
	RoleRefName string
	Subjects    []*Subject
}

func (crb *ClusterRoleBinding) ToManifest() (any, error) {
	var subjects []map[string]any
	for _, s := range crb.Subjects {
		subjects = append(subjects, map[string]any{
			"kind":      s.Kind,
			"name":      s.Name,
			"namespace": s.Namespace,
		})
	}
	manifest := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "rbac.authorization.k8s.io/v1",
			"kind":       "ClusterRoleBinding",
			"metadata": map[string]any{
				"name": crb.Name,
			},
			"subjects": subjects,
			"roleRef": map[string]any{
				"apiGroup": "rbac.authorization.k8s.io",
				"kind":     "ClusterRole",
				"name":     crb.RoleRefName,
			},
		},
	}
	return manifest, nil
}
