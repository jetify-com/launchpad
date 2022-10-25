package helm

import (
	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/cli"
	valuesLib "helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/getter"
)

// MergeValues merges values from different sources similar to helm CLI.
// The order of precedence is (top is highest)
//
// - set values (--set flag)
// - value files (--values flag)
// - base (cli defaults)
//
// This matches helm CLI, except they don't really have "base"
func MergeValues(
	base map[string]any,
	valueFiles []string,
	values string, // from --set
) (map[string]any, error) {
	v, err := (&valuesLib.Options{
		ValueFiles: valueFiles,       // from --values
		Values:     []string{values}, // from --set
		// If needed:
		// StringValues: stringValues, // from --set-string
		// FileValues:   fileValues,   // from --set-file

	}).MergeValues(getter.All(&cli.EnvSettings{}))
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return mergeMaps(base, v), nil
}

// Copy pasted from https://pkg.go.dev/helm.sh/helm/v3@v3.9.0/pkg/cli/values
// Question: launchpad/deploy.go uses mergo which can merge slices but for
// consistency with helm, maybe we should switch to this?
func mergeMaps(a, b map[string]any) map[string]any {
	out := make(map[string]any, len(a))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		if v, ok := v.(map[string]any); ok {
			if bv, ok := out[k]; ok {
				if bv, ok := bv.(map[string]any); ok {
					out[k] = mergeMaps(bv, v)
					continue
				}
			}
		}
		out[k] = v
	}
	return out
}
