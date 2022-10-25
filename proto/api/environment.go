package api

import (
	"strings"

	"github.com/samber/lo"
	"golang.org/x/exp/maps"
)

func (e Environment) ImageTagPrefix() string {
	if e == Environment_PROD {
		return "prod-"
	}
	if e == Environment_DEV {
		return "dev-"
	}
	if e == Environment_STAGING {
		return "staging-"
	}
	return ""
}
func ValidLowercaseEnvironments() []string {
	c := maps.Clone(Environment_name)
	delete(c, int32(Environment_NONE))
	return lo.Map(
		maps.Values(c),
		func(s string, _ int) string { return strings.ToLower(s) },
	)
}

func IsValidEnvironment(s string) bool {
	return lo.Contains(ValidLowercaseEnvironments(), strings.ToLower(s))
}

func EnvironmentFromLowercaseString(s string) Environment {
	return Environment(Environment_value[strings.ToUpper(s)])
}

func (e Environment) ToLower() string {
	return strings.ToLower(e.String())
}
