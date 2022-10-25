package komponents

import (
	"github.com/pkg/errors"
	"go.jetpack.io/launchpad/pkg/reaktor"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// JobWithSpec implements interface Resource (compile-time check)
var _ reaktor.Resource = (*JobWithSpec)(nil)

type JobWithSpec struct {
	Command                 []string
	Name                    string
	Namespace               string
	Labels                  map[string]string
	PodMetadata             map[string]any
	PodSpec                 map[string]any
	BackoffLimit            int
	TTLSecondsAfterFinished int
	manifest                *unstructured.Unstructured
}

func (j *JobWithSpec) ToManifest() (any, error) {
	// see note in Job.ToManifest about this caching.
	if j.manifest != nil {
		return j.manifest, nil
	}

	job := &Job{
		Name:                    j.Name,
		Namespace:               j.Namespace,
		Labels:                  j.Labels,
		PodMetadata:             j.PodMetadata,
		BackoffLimit:            j.BackoffLimit,
		TTLSecondsAfterFinished: j.TTLSecondsAfterFinished,
	}

	untypedManifest, err := job.ToManifest()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	manifest := untypedManifest.(*unstructured.Unstructured)

	podSpec, err := transformPodSpec(j.PodSpec, corev1.RestartPolicyNever, j.Command)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	content := manifest.UnstructuredContent()
	err = unstructured.SetNestedField(content, podSpec, "spec", "template", "spec")
	if err != nil {
		return nil, errors.WithStack(err)
	}
	manifest.SetUnstructuredContent(content)

	j.manifest = manifest
	return manifest, nil
}
