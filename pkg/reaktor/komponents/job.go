package komponents

import (
	"go.jetpack.io/launchpad/pkg/reaktor"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Job struct {
	// TODO: have a way to validate arguments
	Name                    string // TODO: ensure it's a valid k8s name
	Namespace               string
	Labels                  map[string]string
	ContainerImage          string
	Command                 []string
	EnvConfig               EnvConfig
	PodMetadata             map[string]any
	BackoffLimit            int
	TTLSecondsAfterFinished int
	// For tests:
	patchForTest map[string]any

	manifest *unstructured.Unstructured
}

// Job implements interface Resource (compile-time check)
var _ reaktor.Resource = (*Job)(nil)

func (j *Job) ToManifest() (any, error) {
	// Since names are automatically generated, we cache the manifest so a second
	// call returns the same result as before.
	// For now this behavior is convenient, but it has implications on how easy
	// or hard it is to launch a new *instance* of the job, vs modifying an instance
	// that already exists.
	// We should discuss in more detail what the right programming model is.
	if j.manifest != nil {
		return j.manifest, nil
	}

	// Override zero for backwards compatibility
	if j.TTLSecondsAfterFinished == 0 {
		// Let jobs exist for 24 hours (for debugging), but clean up
		// after that
		j.TTLSecondsAfterFinished = 24 * 60 * 60
	}

	// TODO: Don't add "envFrom", "volumeMounts" or "volumes" to the manifest when they are not needed.
	j.manifest = &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "batch/v1",
			"kind":       "Job",
			"metadata": map[string]any{
				"name":      j.Name,
				"namespace": j.Namespace, // TODO: make optional
				"labels":    j.Labels,
			},
			"spec": map[string]any{
				"ttlSecondsAfterFinished": j.TTLSecondsAfterFinished,
				// Setting backoffLimit to 0 or MaxRetry based on withCheckPointing flag
				"backoffLimit": j.BackoffLimit,
				"template": map[string]any{
					"spec": map[string]any{
						"containers": []map[string]any{
							{
								"name":  j.Name,
								"image": j.ContainerImage,
								// TODO: set only if command is not empty so that the default
								// is using the "entrypoint" from the container
								"command": j.Command,
								// TODO: Add support for envConfigs that provide the actual envvar values, instead of using `envFrom`
								"envFrom":      j.envConfig().ToEnvFrom(),
								"volumeMounts": j.envConfig().ToVolumeMounts(),
							},
						},
						"volumes": j.envConfig().ToVolumes(),
						// Defaulting to "Never" since jobs might not be idempotent
						// Note that this setting applies only to the pod not being
						// restarted. The JobController may still start new pods
						// since the Job has not completed. We use backoffLimit
						// to ensure the pods are not restarted again.
						"restartPolicy": "Never",
					},
					"metadata": j.PodMetadata,
				},
			},
		},
	}

	// merge the patch
	j.manifest.Object = j.mergePatch(j.manifest.Object)

	return j.manifest, nil
}

func (j *Job) envConfig() EnvConfig {
	if j == nil || j.EnvConfig == nil {
		return &EmptyEnvConfig{}
	}
	return j.EnvConfig
}

// PatchForTest is an api for tests. It is too fast-and-loose to use in
// production. Please use more structured APIs for prod.
//
// patchMapper patchMapper make me a patch
// find me a find
// catch me a catch
func (j *Job) PatchForTest(patchMapper map[string]any) {
	j.patchForTest = patchMapper
}

// If other components also need to patch for tests, we can move this into a file
// and interface where it can be used by other reaktor komponents. Will defer
// doing that until needed by at least one more komponent.
func (j *Job) mergePatch(
	manifestObj map[string]any,
) map[string]any {
	if j.patchForTest == nil {
		return manifestObj
	}

	for k, v := range j.patchForTest {
		if prevVal, ok := manifestObj[k]; ok {

			// If generalizing this function to work for more test-cases, here
			// is one way of extending it:
			// - if not a map, then see if it is a list type, and merge the lists.
			// - otherwise it is a scalar value which is overridden by the patchForTest value.
			if prevValMap, ok := prevVal.(map[string]any); ok {
				manifestObj[k] = mergeMap(prevValMap, v.(map[string]any))
			}
		} else {
			manifestObj[k] = v
		}
	}
	return manifestObj
}

// TODO move to pkg/goutil/mapstr, but first need to check if we envision moving
// that entire package to opensource/ ?
func mergeMap(m1 map[string]any, m2 map[string]any) map[string]any {
	result := map[string]any{}
	for k1, v1 := range m1 {
		result[k1] = v1
	}
	for k2, v2 := range m2 {
		result[k2] = v2
	}
	return result
}
