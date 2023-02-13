package jetconfig

import (
	"strings"

	"github.com/samber/lo"
	"go.jetpack.io/launchpad/goutil/errorutil"
	"gopkg.in/yaml.v3"
)

func validInstanceTypes() []string {
	return lo.Map(lo.Values(InstanceType_name)[1:], func(v string, _ int) string {
		return strings.ToLower(v)
	})
}

func (i *InstanceType) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.ScalarNode {
		return errorutil.NewUserErrorf(
			"Invalid instance type. Valid types are %s",
			strings.Join(validInstanceTypes(), ", "),
		)
	}
	it, ok := InstanceType_value[strings.ToUpper(value.Value)]
	if !ok || InstanceType(it) == InstanceType_UNKNOWN {
		return errorutil.NewUserErrorf(
			"Invalid instance type: \"%s\". Valid instance types are: %s",
			value.Value,
			strings.Join(validInstanceTypes(), ", "),
		)
	}
	*i = InstanceType(it)
	return nil
}

func (i *InstanceType) MarshalYAML() (any, error) {
	return strings.ToLower(i.String()), nil
}

func (i *InstanceType) Compute() string {
	if i == nil {
		i = lo.ToPtr(InstanceType_MICRO)
	}
	switch *i {
	case InstanceType_NANO:
		return "125m"
	case InstanceType_MICRO:
		return "250m"
	case InstanceType_SMALL:
		return "500m"
	case InstanceType_MEDIUM:
		return "1000m"
	case InstanceType_MEDIUM_PLUS:
		return "1500m"
	default:
		return ""
	}
}

func (i *InstanceType) Memory() string {
	if i == nil {
		i = lo.ToPtr(InstanceType_MICRO)
	}
	switch *i {
	case InstanceType_NANO:
		return "256Mi"
	case InstanceType_MICRO:
		return "512Mi"
	case InstanceType_SMALL:
		return "1024Mi"
	case InstanceType_MEDIUM:
		return "2048Mi"
	case InstanceType_MEDIUM_PLUS:
		return "3072Mi"
	default:
		return ""
	}
}
