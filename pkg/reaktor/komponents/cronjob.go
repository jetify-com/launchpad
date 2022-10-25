package komponents

import (
	"go.jetpack.io/launchpad/pkg/reaktor"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type CronJob struct {
	Name           string
	Namespace      string
	Labels         map[string]string
	ContainerImage string
	Schedule       string // the crontab string
	Command        []string
	EnvConfig      EnvConfig
	PodMetadata    map[string]any
}

// CronJob implements interface Resource (compile-time check)
var _ reaktor.Resource = (*CronJob)(nil)

func (j *CronJob) ToManifest() (any, error) {
	// TODO: should we cache the manifest like Job does?

	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "batch/v1",
			"kind":       "CronJob",
			"metadata": map[string]any{
				"name":      j.Name,
				"namespace": j.Namespace,
				"labels":    j.Labels,
			},
			"spec": map[string]any{
				"schedule": j.Schedule,
				"jobTemplate": map[string]any{
					"spec": map[string]any{
						"template": map[string]any{
							"spec": map[string]any{
								"containers": []map[string]any{
									{
										"name":         j.Name,
										"image":        j.ContainerImage,
										"command":      j.Command,
										"envFrom":      j.envConfig().ToEnvFrom(),
										"volumeMounts": j.envConfig().ToVolumeMounts(),
									},
								},
								"volumes": j.envConfig().ToVolumes(),
							},
							"metadata": j.PodMetadata,
						},
					},
				},
			},
		},
	}, nil
}

func (j *CronJob) envConfig() EnvConfig {
	if j == nil || j.EnvConfig == nil {
		return &EmptyEnvConfig{}
	}
	return j.EnvConfig
}
