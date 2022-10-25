package komponents

import (
	"go.jetpack.io/launchpad/pkg/reaktor"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// HPA implements interface Resource (compile-time check)
var _ reaktor.Resource = (*HorizontalPodAutoscaler)(nil)

type ScaleTargetRef struct {
	ApiVersion string
	Kind       string
	Name       string
}

type MetricResource struct {
	Name   string
	Target map[string]any
}

type Metric struct {
	Type     string
	Resource MetricResource
}

type HorizontalPodAutoscaler struct {
	Name           string
	Namespace      string
	Labels         map[string]string
	ScaleTargetRef ScaleTargetRef
	MinReplicas    int
	MaxReplicas    int
	Metrics        []Metric
}

func (hpa *HorizontalPodAutoscaler) ToManifest() (any, error) {

	var metrics []map[string]any
	for _, metric := range hpa.Metrics {
		avgUtilization, ok := metric.Resource.Target["averageUtilization"]
		if !ok {
			avgUtilization = ""
		}

		metrics = append(metrics, map[string]any{
			"type": "Resource",
			"resource": map[string]any{
				"name": metric.Resource.Name,
				"target": map[string]any{
					"type": metric.Resource.Target["type"],
					// Note, I'm being lazy here. This may not always be defined.
					// Please refactor this code if needed.
					"averageUtilization": avgUtilization,
				},
			},
		})
	}

	manifest := &unstructured.Unstructured{
		Object: map[string]any{
			// This version is deprecated in 1.24 but we still need to support 1.23
			// Maybe we can get cluster info and choose right version?
			"apiVersion": "autoscaling/v2beta2",
			"kind":       "HorizontalPodAutoscaler",
			"metadata": map[string]any{
				"name":      hpa.Name,
				"namespace": hpa.Namespace,
				"labels":    hpa.Labels,
			},
			"spec": map[string]any{
				"scaleTargetRef": map[string]any{
					"apiVersion": hpa.ScaleTargetRef.ApiVersion,
					"kind":       hpa.ScaleTargetRef.Kind,
					"name":       hpa.ScaleTargetRef.Name,
				},
				"minReplicas": hpa.MinReplicas,
				"maxReplicas": hpa.MaxReplicas,
				"metrics":     metrics,
			},
		},
	}
	return manifest, nil
}
