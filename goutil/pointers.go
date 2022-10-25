package goutil

import "github.com/samber/lo"

// Coalesce like lo.Coalesce, but without the second return value
func Coalesce[T comparable](v ...T) T {
	res, _ := lo.Coalesce(v...)
	return res
}
