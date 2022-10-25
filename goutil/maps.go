package goutil

import "fmt"

// DigDelete deletes a nested key in a multi dimensional map. All value types
// in the key path must be of the same type as the top level map, with the
// exception of the value associated with the key to be deleted
// (that value can be any type). This is a limitation because go does not do
// covariance (map[int]int is not a map[int]any)
//
// For example:
//
// map[int]any{1: map[int]any{2: map[int]int{3: 5}}} is a valid map
// because all value types are the same;
//
// map[int]any{1: map[int]any{2: map[int]int{3: 5}}} is not a valid map because
// last map is a different type
func DigDelete[K comparable, V any](m map[K]V, path ...K) bool {
	for i, p := range path {
		v, ok := m[p]
		if !ok {
			break
		}

		if i == len(path)-1 {
			delete(m, p)
			return true
		}

		m, ok = any(v).(map[K]V)
		if !ok {
			break
		}
	}
	return false
}

// FilterStringKeyMap removes nil values from a map recursively. It
// mutates the passed in value and returns it as well for convenience.
func FilterStringKeyMap(m map[string]any) map[string]any {
	for key, val := range m {
		if subMap, isMap := val.(map[string]any); isMap {
			FilterStringKeyMap(subMap)
		} else if val == nil {
			delete(m, key)
		}
	}
	return m
}

func Entries(m map[string]string) []string {
	r := []string{}
	for name, value := range m {
		r = append(r, fmt.Sprintf("%s=%s", name, value))
	}
	return r
}
