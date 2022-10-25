package komponents

import (
	"strings"

	"go.jetpack.io/launchpad/pkg/reaktor"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const certmanagerNamespace = "cert-manager"

type Host struct {
	Hostname  string
	Namespace string
}

func HostInDefaultNamespace(h string) *Host {
	return &Host{
		Hostname:  h,
		Namespace: certmanagerNamespace,
	}
}

// Secret implements interface Resource (compile-time check)
var _ reaktor.Resource = (*Host)(nil)

// ToManifest is only structured for reading. For creating secrets more work
// needs to be done.
func (ns *Host) ToManifest() (any, error) {
	name := strings.ReplaceAll(ns.Hostname, ".", "-")
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "getambassador.io/v3alpha1",
			"kind":       "Host",
			"metadata": map[string]any{
				"name":      name,
				"namespace": ns.Namespace,
			},
			"spec": map[string]any{
				"hostname": ns.Hostname,
				"acmeProvider": map[string]any{
					"email": "eng@jetpack.io",
				},
				"tls": map[string]any{
					"min_tls_version": "v1.2",
					"alpn_protocols":  "h2,http/1.1",
				},
			},
		},
	}, nil
}
