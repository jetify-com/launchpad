package komponents

import (
	"go.jetpack.io/launchpad/pkg/reaktor"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Service implements interface Resource (compile-time check)
var _ reaktor.Resource = (*Service)(nil)

type Port struct {
	Name       string
	Port       int
	Protocol   string
	TargetPort string
}

type Service struct {
	Name           string
	Namespace      string
	Labels         map[string]string
	Ports          []Port
	SelectorLabels map[string]string
	Type           string // TODO make enum: ClusterIP, NodePort, LoadBalancer, ExternalName
}

func (svc *Service) ToManifest() (any, error) {
	var ports []map[string]any
	for _, port := range svc.Ports {
		ports = append(ports, map[string]any{
			"name":       port.Name,
			"port":       port.Port,
			"protocol":   port.Protocol,
			"targetPort": port.TargetPort,
		})
	}

	manifest := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata": map[string]any{
				"name":      svc.Name,
				"namespace": svc.Namespace,
				"labels":    svc.Labels,
			},
			"spec": map[string]any{
				"type":     svc.Type,
				"ports":    ports,
				"selector": svc.SelectorLabels,
			},
		},
	}
	return manifest, nil
}
