package reaktor

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func DeploymentGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
}

func PodGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
}

func CronJobGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "cronjobs"}
}

func ServiceAccountGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "", Version: "v1", Resource: "serviceaccounts"}
}

func SecretGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}
}

func AmbassadorMappingGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "getambassador.io", Version: "v3alpha1", Resource: "mappings"}
}

func ServiceGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services"}
}
