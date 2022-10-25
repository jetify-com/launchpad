package komponents

import (
	"go.jetpack.io/launchpad/pkg/reaktor"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Experiment to see if we prefer to used the typed objects from
// the typed K8s APIs.
// So far:
// Pros: It's typed, so you get compile time checking, completion, docs in IDE
// Cons: Feels slow to translate. With the untyped case I can easily look at
//
//	a YAML and write the corresponding struct very quickly. With the
//	typed API I find myself trying to figure out in which go package
//	a given type is defined, and making sure we're importing it
type TypedJob struct {
	// TODO: have a way to validate arguments
	Name           string // TODO: ensure it's a valid k8s name
	Namespace      string
	ContainerImage string
	Command        []string
}

// Job implements interface Resource (compile-time check)
var _ reaktor.Resource = (*TypedJob)(nil)

func (j *TypedJob) ToManifest() (any, error) {
	var ttl int32 = 24 * 60 * 60
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      j.Name,
			Namespace: j.Namespace,
		},
		Spec: batchv1.JobSpec{
			// Ugh, int32 pointer literals are a pain in the butt and require
			// a helper var as above
			TTLSecondsAfterFinished: &ttl,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:    j.Name,
							Image:   j.ContainerImage,
							Command: j.Command,
						},
					},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
		},
	}, nil
}
