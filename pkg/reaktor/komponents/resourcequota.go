package komponents

import (
	"go.jetpack.io/launchpad/pkg/reaktor"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ResourceQuota implements interface Resource (compile-time check)
var _ reaktor.Resource = (*ResourceQuota)(nil)

type Quota struct {
	Pods string
}

type ResourceQuota struct {
	Name      string
	Namespace string
	Quota     *Quota
}

func (rb *ResourceQuota) ToManifest() (any, error) {
	manifest := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ResourceQuota",
			"metadata": map[string]any{
				"name":      rb.Name,
				"namespace": rb.Namespace,
			},
			"spec": map[string]any{
				"hard": map[string]any{
					"pods": rb.Quota.Pods,
				},
			},
		},
	}
	return manifest, nil
}
