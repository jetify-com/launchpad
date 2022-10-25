package komponents

import (
	"go.jetpack.io/launchpad/pkg/reaktor"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Passthru struct {
	JsonManifest []byte // json
}

// Passthru implements interface Resource (compile-time check)
var _ reaktor.Resource = (*Passthru)(nil)

func (p *Passthru) ToManifest() (any, error) {
	u := unstructured.Unstructured{}
	_, _, err := unstructured.UnstructuredJSONScheme.Decode(p.JsonManifest, nil, &u)
	return u, err
}
