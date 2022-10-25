package semver

import (
	"github.com/pkg/errors"
	"golang.org/x/mod/semver"
)

var errInvalidValue = errors.New("Invalid semver value")

// Compare returns:
// -1 if v < w, 0 if v == w, or +1 if v > w.
func Compare(v string, w string) (int, error) {
	val1 := "v" + v
	val2 := "v" + w

	if !semver.IsValid(val1) {
		return 0, errors.Wrapf(errInvalidValue, "first value: %s", val1)
	}

	if !semver.IsValid(val2) {
		return 0, errors.Wrapf(errInvalidValue, "second value: %s", val2)
	}

	return semver.Compare(val1, val2), nil
}

func IsValid(val string) bool {
	return semver.IsValid("v" + val)
}
