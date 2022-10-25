package komponents

import (
	"go.jetpack.io/launchpad/pkg/reaktor"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ reaktor.Resource = (*Deployment)(nil)

type Selector struct {
	MatchLabels map[string]string `json:"matchLabels"`
}

type Template struct {
	Metadata map[string]any `json:"metadata"`
	Spec     map[string]any `json:"spec"`
}

type DeploymentSpec struct {
	Selector Selector `json:"selector"`
	Template Template `json:"template"`
}

type Deployment struct {
	Name      string
	Namespace string
	Labels    map[string]string
	Spec      DeploymentSpec
}

func (d *Deployment) ToManifest() (any, error) {
	manifest := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]any{
				"name":      d.Name,
				"namespace": d.Namespace,
				"labels":    d.Labels,
			},
			"spec": d.Spec,
		},
	}

	return manifest, nil
}
