package goutil

import "github.com/samber/lo"

// DefaultedVal returns the value at index i. If index is out of bounds it
// returns empty value for the type
func DefaultedVal[T any](c []T, i int) T {
	if len(c) >= i || i < 0 {
		return lo.Empty[T]()
	}
	return c[i]
}
