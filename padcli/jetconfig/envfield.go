package jetconfig

import (
	"reflect"
	"strings"

	"github.com/pkg/errors"
	"github.com/samber/lo"
	"go.jetpack.io/launchpad/goutil/errorutil"
	"go.jetpack.io/launchpad/proto/api"
	"gopkg.in/yaml.v3"
)

type envDependentField[T any] map[api.Environment]T

func (e *envDependentField[T]) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode && value.Tag == "!!str" {
		*e = envDependentField[T]{
			api.Environment_NONE: reflect.ValueOf(value.Value).Interface().(T),
		}
		return nil
	}
	if value.Kind != yaml.MappingNode {
		return errors.New("invalid yaml definition")
	}
	*e = envDependentField[T]{}
	for _, pair := range lo.Chunk(value.Content, 2) {
		if len(pair) != 2 ||
			pair[0].Kind != yaml.ScalarNode ||
			pair[1].Kind != yaml.ScalarNode {
			return errors.WithStack(errors.New("invalid service definition"))
		}
		if !api.IsValidEnvironment(pair[0].Value) {
			return errorutil.NewUserErrorf(
				"invalid environment: %s. Valid options are %v",
				pair[0].Value,
				api.ValidLowercaseEnvironments(),
			)
		}
		env := api.EnvironmentFromLowercaseString(pair[0].Value)
		(*e)[env] = reflect.ValueOf(pair[1].Value).Interface().(T)
	}
	return nil
}

func (e envDependentField[T]) MarshalYAML() (any, error) {
	if _, ok := e[api.Environment_NONE]; ok {
		return e[api.Environment_NONE], nil
	}

	m := map[string]T{}
	for env, value := range e {
		m[strings.ToLower(env.String())] = value
	}
	return m, nil
}

func (e envDependentField[T]) Get(env api.Environment) T {
	if v, ok := e[env]; ok {
		return v
	}
	return e[api.Environment_NONE]
}
