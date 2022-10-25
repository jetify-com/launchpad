package reaktor

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	yamlpkg "k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	cachetools "k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

const reaktorFieldManager string = "reaktor"

// See constructors.go for how to create
type Reaktor struct {
	dynamicClient dynamic.Interface // Interface
	factory       cmdutil.Factory   // Interface
	mapper        meta.RESTMapper   // Interface
}

func (k *Reaktor) Apply(ctx context.Context, r Resource) (*unstructured.Unstructured, error) {
	manifest, err := ToManifest(r)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get manifest")
	}

	result, err := k.applyUnstructured(ctx, manifest)
	return result, errors.WithStack(err)
}

func (k *Reaktor) ApplyYaml(ctx context.Context, yaml []byte) (*unstructured.Unstructured, error) {
	u := &unstructured.Unstructured{}
	// Why use `UnstructuredJSONScheme`?
	// because from its source code comment:
	// NewDecodingSerializer adds YAML decoding support to a serializer that supports JSON.
	//
	// sigh
	//
	// reference:
	// https://ymmt2005.hatenablog.com/entry/2020/04/14/An_example_of_using_dynamic_client_of_k8s.io/client-go
	dec := yamlpkg.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	_, _, err := dec.Decode(yaml, nil, u)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	result, err := k.applyUnstructured(ctx, u)
	return result, errors.WithStack(err)
}

func (k *Reaktor) applyUnstructured(ctx context.Context, manifest *unstructured.Unstructured) (*unstructured.
	Unstructured,
	error) {
	resource, err := k.ToKubeResource(ctx, manifest)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get kube resource")
	}

	json, err := manifest.MarshalJSON()
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal JSON")
	}

	response, err := resource.Patch(
		ctx,
		manifest.GetName(),
		types.ApplyPatchType, // Use server side apply
		json,
		metav1.PatchOptions{
			FieldManager: reaktorFieldManager,
		})
	if err != nil {
		return nil, errors.Wrap(err, "failed to patch resource")
	}

	return response, nil
}

func (k *Reaktor) Get(
	ctx context.Context,
	gvr schema.GroupVersionResource,
	name string,
	ns string,
) (*unstructured.Unstructured, error) {
	u, err := k.dynamicClient.Resource(gvr).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
	return u, errors.WithStack(err)
}

func (k *Reaktor) List(
	ctx context.Context,
	gvr schema.GroupVersionResource,
	ns string,
	listOptions metav1.ListOptions,
) (*unstructured.UnstructuredList, error) {
	u, err := k.dynamicClient.Resource(gvr).Namespace(ns).List(ctx, listOptions)
	return u, errors.WithStack(err)
}

func (k *Reaktor) GetByResource(ctx context.Context, r Resource) (*unstructured.Unstructured, error) {
	// Food for thought:
	// If the creating resources is inspired by React, should the way we query for
	// resources be inspired by GraphQL in any way?
	manifest, err := ToManifest(r)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	resource, err := k.ToKubeResource(ctx, manifest)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return resource.Get(ctx, manifest.GetName(), metav1.GetOptions{})
}

func (k *Reaktor) Delete(ctx context.Context, r Resource) error {
	manifest, err := ToManifest(r)
	if err != nil {
		return errors.WithStack(err)
	}

	resource, err := k.ToKubeResource(ctx, manifest)
	if err != nil {
		return errors.WithStack(err)
	}

	return resource.Delete(ctx, manifest.GetName(), metav1.DeleteOptions{})
}

func (k *Reaktor) DeleteCollection(
	ctx context.Context,
	gvr schema.GroupVersionResource,
	ns string,
	labelSelector string, // e.g. "app=hello-world"
) error {
	return errors.WithStack(
		k.dynamicClient.Resource(gvr).Namespace(ns).DeleteCollection(
			ctx, metav1.DeleteOptions{}, metav1.ListOptions{LabelSelector: labelSelector}))
}

func (k *Reaktor) ToKubeResource(
	ctx context.Context,
	manifest *unstructured.Unstructured,
) (dynamic.ResourceInterface, error) {
	gvr, err := GroupVersionResource(k.mapper, manifest)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get GVR")
	}

	// TODO: in the future we might need to handle cluster-wide resources vs
	// namespaced resources differently.
	ns := manifest.GetNamespace()
	if ns == "" {
		return k.dynamicClient.Resource(gvr), nil
	} else {
		return k.dynamicClient.Resource(gvr).Namespace(ns), nil
	}
}

func ToManifest(r Resource) (*unstructured.Unstructured, error) {
	i, err := r.ToManifest()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	switch value := i.(type) {
	case Resource:
		return ToManifest(value)
	case *unstructured.Unstructured:
		return value, nil
	case unstructured.Unstructured:
		return &value, nil
	default:
		// Assume we're dealing with a kubernetes typed object otherwise:
		return KubeStructToManifest(value)
	}
}

// ToJSONManifest is a convenience method that returns the JSON
// representation of the Resource's manifest.
func ToJSONManifest(r Resource) ([]byte, error) {
	m, err := ToManifest(r)
	if err != nil {
		return nil, errors.Wrap(err, "failed to created manifest")
	}

	return m.MarshalJSON()
}

func (k *Reaktor) WatchUntil(
	ctx context.Context,
	unstructuredObj *unstructured.Unstructured,
	// untilFunc should return true if Reaktor should stop watching.
	untilFunc func(e watch.Event) (bool, error),
) (*watch.Event, error) {

	// Get a REST Client
	gvk := unstructuredObj.GroupVersionKind()
	mapping, err := k.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	restClient, err := k.factory.UnstructuredClientForMapping(mapping)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	// get the Resource
	gvr, err := GroupVersionResource(k.mapper, unstructuredObj)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	objName := unstructuredObj.GetName()
	selector, err := fields.ParseSelector(fmt.Sprintf("metadata.name=%s", objName))
	if err != nil {
		return nil, errors.WithStack(err)
	}

	lw := cachetools.NewListWatchFromClient(
		restClient,
		gvr.Resource,
		unstructuredObj.GetNamespace(),
		selector,
	)

	w, err := watchtools.UntilWithSync(
		ctx,
		lw,
		unstructuredObj,
		nil, // precondition
		untilFunc,
	)
	return w, errors.WithStack(err)
}
