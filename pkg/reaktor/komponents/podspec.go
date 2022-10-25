package komponents

import (
	"fmt"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// transformPodSpec edits the provided podSpec so that it can be used in another
// komponent. The edits consist of 3 things:
//  1. Set the restartPolicy to the provided restart policy.
//  2. Drop the nodeName, if any.
//  3. Get the first container and update its command to the provided command.
//  4. Set the spec's containers to the single updated container from (2). Note
//     that other containers will be dropped.
func transformPodSpec(
	podSpec map[string]any,
	restartPolicy corev1.RestartPolicy,
	command []string,
) (map[string]any, error) {
	// Set restartPolicy.
	err := unstructured.SetNestedField(podSpec, string(restartPolicy), "restartPolicy")
	if err != nil {
		return nil, errors.WithStack(err)
	}

	// Remove nodeName, to avoid the potential of referring to a non-existing node.
	unstructured.RemoveNestedField(podSpec, "nodeName")

	// Get the first container.
	untypedContainers, found, err := unstructured.NestedFieldCopy(podSpec, "containers")
	if !found {
		return nil, errors.Wrap(err, "expect to find containers in podSpec")
	}
	containers := untypedContainers.([]any)
	if len(containers) > 1 {
		fmt.Println("WARNING: more than 1 container found in pod spec. Using first one")
	}
	firstContainer := containers[0].(map[string]any)

	// Update the command in the container.
	err = unstructured.SetNestedStringSlice(firstContainer, command, "command")
	if err != nil {
		return nil, errors.WithStack(err)
	}

	// Update the PodSpec with the updated container.
	containers[0] = firstContainer
	err = unstructured.SetNestedField(podSpec, containers, "containers")
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return podSpec, nil
}
