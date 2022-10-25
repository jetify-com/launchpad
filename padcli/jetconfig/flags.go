package jetconfig

import (
	"fmt"

	"github.com/pkg/errors"
)

type FlagSet map[string]any

func (f FlagSet) GetValueAsString(name string) (string, error) {
	source := f[name]
	if source == nil {
		return "", errors.WithStack(errors.Errorf("no value defined for flag '%s'", name))
	}
	return formatValueAsString(source), nil
}

func (f FlagSet) GetValueAsStringSlice(name string) ([]string, error) {
	source := f[name]
	if source == nil {
		return nil, errors.WithStack(errors.Errorf("no value defined for flag '%s'", name))
	}

	if values, ok := f[name].([]string); ok {
		return values, nil
	}

	if v, ok := source.([]any); ok {
		var values []string
		for _, value := range v {
			values = append(values, formatValueAsString(value))
		}
		return values, nil
	} else {
		return []string{formatValueAsString(source)}, nil
	}
}

func formatValueAsString(value any) string {
	if s, ok := value.([]any); ok {
		if len(s) == 0 {
			return ""
		}
		if len(s) == 1 {
			return formatValueAsString(s[0])
		}
	}
	return fmt.Sprint(value)
}
