package goutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilterStringKeyMap(t *testing.T) {
	t.Run("TestFilterStringKeyMapEmpty", func(t *testing.T) {
		assert.Equal(
			t,
			map[string]any{},
			FilterStringKeyMap(map[string]any{}),
		)
	})
	t.Run("TestFilterStringKeyMapWithValues", func(t *testing.T) {
		assert.Equal(
			t,
			map[string]any{
				"emptyStringKey":    "",
				"emptyIntKey":       0,
				"nonEmptyStringKey": "foo",
				"nonEmptyIntKey":    4,
				"map": map[string]any{
					"innerEmptyKey":           "",
					"innerNonEmptyKey":        "inner",
					"innerNilIfEmptyNonEmpty": "foo",
				},
			},
			FilterStringKeyMap(map[string]any{
				"emptyStringKey":    "",
				"nilVal":            nil,
				"emptyIntKey":       0,
				"nonEmptyStringKey": "foo",
				"nonEmptyIntKey":    4,
				"map": map[string]any{
					"innerEmptyKey":           "",
					"innerNilKey":             nil,
					"innerNonEmptyKey":        "inner",
					"innerNilIfEmpty":         nil,
					"innerNilIfEmptyNonEmpty": "foo",
				},
			}),
		)
	})
}
