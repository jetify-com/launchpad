package goutil

import "github.com/samber/lo"

// NonEmptyFilter to be used with lo.Filter
func NonEmptyFilter[T comparable](t T, _ int) bool {
	return t != lo.Empty[T]()
}
