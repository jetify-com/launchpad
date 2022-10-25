package komponents

import (
	"github.com/pkg/errors"
	"go.jetpack.io/launchpad/pkg/reaktor"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// CronJobWithSpec represents a CronJob which is created
// using an existing Pod's spec as its base. This means
// it will re-use the image and environment information
// from the existing pod (via its spec).
// This komponent is analogous to JobWithSpec.
type CronJobWithSpec struct {
	Name        string
	Namespace   string
	Labels      map[string]string
	Schedule    string
	Command     []string
	PodMetadata map[string]any
	PodSpec     map[string]any
}

// CronJobWithSpec implements interface Resource (compile-time check)
var _ reaktor.Resource = (*CronJobWithSpec)(nil)

func (j *CronJobWithSpec) ToManifest() (any, error) {
	// TODO: should we cache? I think yes.

	cj := &CronJob{
		Name:        j.Name,
		Namespace:   j.Namespace,
		Labels:      j.Labels,
		Schedule:    j.Schedule,
		PodMetadata: j.PodMetadata,
	}
	cjManifest, err := cj.ToManifest()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	manifest := cjManifest.(*unstructured.Unstructured)

	podSpec, err := transformPodSpec(j.PodSpec, corev1.RestartPolicyOnFailure, j.Command)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	// Replace the pod spec on the manifest with the one we inherited/prepared.
	content := manifest.UnstructuredContent()
	err = unstructured.SetNestedField(
		content, podSpec, "spec", "jobTemplate", "spec", "template", "spec")
	if err != nil {
		return nil, errors.WithStack(err)
	}
	manifest.SetUnstructuredContent(content)

	return manifest, nil
}
