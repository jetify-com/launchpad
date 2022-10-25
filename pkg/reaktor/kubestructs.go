package reaktor

import (
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Takes a typed structure from the kubernetes api (i.e. batchv1.Job, etc)
func KubeStructToManifest(kubeStruct any) (*unstructured.Unstructured, error) {
	untyped, err := runtime.DefaultUnstructuredConverter.ToUnstructured(kubeStruct)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	manifest := &unstructured.Unstructured{
		Object: untyped,
	}
	return manifest, nil
}

func GroupVersionResource(mapper meta.RESTMapper, manifest *unstructured.Unstructured) (schema.GroupVersionResource, error) {
	gvk := manifest.GroupVersionKind()
	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return schema.GroupVersionResource{}, errors.Wrap(err, "failed to get RESTMapping")
	}

	return mapping.Resource, nil
}
