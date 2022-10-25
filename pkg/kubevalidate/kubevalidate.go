package kubevalidate

import (
	"encoding/base32"
	"fmt"
	"hash/fnv"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/rand"

	"k8s.io/apimachinery/pkg/util/validation"
)

// nameMaxLength is the longest valid string that will be included in the name.
// Names longer than nameMaxLength should be chopped.
const nameMaxLength = 63

// https://kubernetes.io/docs/concepts/workloads/controllers/cron-jobs/
const cronjobNameMaxLength = 52

// KubernetesSlugLength is the length used in kubernetes code
// https://github.com/kubernetes/apiserver/blob/master/pkg/storage/names/generate.go#L45-L53
const kubernetesSlugLength = 5

// Internally use as much of the validation code from "k8s.io/apimachinery/pkg/util/validation"
// as possible.

// TODO: Should we expand this package beyond kubernetes and move the docker name
// validation logic we have in pkg/docker/docker.go in here as well?

// By creating a singleton we can pre-compile all the regex expressions.
var defaultValidator *validator = compile()

type validator struct {
	badPrefix         *regexp.Regexp
	badSuffix         *regexp.Regexp
	nilSeparator      *regexp.Regexp
	badSeparator      *regexp.Regexp
	repeatedSeparator *regexp.Regexp
	badCharacters     *regexp.Regexp

	nonAlphabeticPrefix *regexp.Regexp
}

func compile() *validator {
	return &validator{
		badPrefix:         regexp.MustCompile(`^[^[:alnum:]]+`),
		badSuffix:         regexp.MustCompile(`[^[:alnum:]]+$`),
		nilSeparator:      regexp.MustCompile(`[']+`),
		badSeparator:      regexp.MustCompile(`([[:alnum:]])[^[:alnum:]._-]+([[:alnum:]])`),
		repeatedSeparator: regexp.MustCompile(`([._-])[._-]*`),
		badCharacters:     regexp.MustCompile(`[^[:alnum:]._-]+`),

		nonAlphabeticPrefix: regexp.MustCompile(`^[^[:alpha:]]+`),
	}
}

var ErrInvalidName = errors.New("Invalid Name")

// ToIdentifier attempts to convert the provided string into an alternate version
// that:
//   - Contains only ASCII alphanumeric characters, or an allowed separator ('-', '_', '.')
//   - Starts and ends with an alphanumeric character and *not* a separator
//   - Has each chunk of alphanumeric characters separated by at most one separator
//     (in other words, separators don't repeat)
//
// This function is often helpful as a first pass before stricter transformations
func ToIdentifier(s string) string {
	return defaultValidator.toIdentifier(s)
}

func (v *validator) toIdentifier(s string) string {
	// Remove disallowed characters from the beginning of the string:
	s = v.badPrefix.ReplaceAllString(s, "")

	// Remove disallowed characters from the end of the string:
	s = v.badSuffix.ReplaceAllString(s, "")

	// Replace a special subset of separators by the empty string. In particular
	// we want to handle the possesive case in English (i.e. "daniel's") so that
	// it becomes "daniels" instead of "daniel-s"
	s = v.nilSeparator.ReplaceAllString(s, "")

	// Replace bad separators by '-':
	s = v.badSeparator.ReplaceAllString(s, "$1-$2")

	// Remove any disallowed characters left:
	s = v.badCharacters.ReplaceAllString(s, "")

	// If there are multiple permitted separators next to each other, pick just
	// one of them. We default to picking the first one that appears.
	s = v.repeatedSeparator.ReplaceAllString(s, "$1")

	return s
}

// Checks if the given name is a lowercase DNS label as defined in RFC 1123.
func IsValidRFC1123Name(s string) bool {
	errs := validation.IsDNS1123Label(s)
	return len(errs) == 0
}

// Checks if the given name is a lowercase DNS subdomain name as defined in RFC 1123.
func IsValidSubdomain(s string) bool {
	// If we want to make it possible for the client to _explain_ why a given value
	// is not valid, then we'd need to return these error strings
	errs := validation.IsDNS1123Subdomain(s)
	return len(errs) == 0
}

// Best effort attempt to convert the provided string into an alternate version
// that is a lowercase DNS subdomain name as defined in RFC 1123.
// Basically the string must:
//   - Contain lowercase alphanumeric characters, '-' or '.'
//   - It must start and end with an alphanumeric character.
//   - Separators '-' and '.' should be followed by an alphanumeric character.
func ToValidSubdomain(s string) (string, error) {
	return defaultValidator.toValidSubdomain(s)
}

// Implementation that uses the singleton
func (v *validator) toValidSubdomain(s string) (string, error) {
	if IsValidSubdomain(s) {
		return s, nil
	}
	s = ToIdentifier(s)
	s = strings.ToLower(s)

	s = strings.ReplaceAll(s, "_", "-")

	if len(s) == 0 {
		return "", ErrInvalidName
	}

	// TODO: Handle the case when the passed string is larger than the max length
	// of 253 (we need to chomp it, but make sure it still ends in alphanumeric)
	// Depending on the use case it's also unclear whether it's better to chomp
	// at the beginning or end of the string.
	return s, nil
}

// Checks if the given name is a label name as defined by RFC 1035.
func IsValidName(s string) bool {
	// If we want to make it possible for the client to _explain_ why a given value
	// is not valid, then we'd need to return these error strings
	errs := validation.IsDNS1035Label(s)
	return len(errs) == 0
}

func IsValidNameMsgs(s string) []string {
	return validation.IsDNS1035Label(s)
}

// ToValidName attempts to convert the provided string into an alternate version
// that is a label name as defined in RFC 1123.
// Basically the string must:
//   - Contain lowercase alphanumeric characters or '-'
//   - It must start with an alphabetic character
//   - It must end with an alphanumeric character
//   - It must no more than nameMaxLength characters
func ToValidName(s string) (string, error) {
	// truncateWithSlug: false for legacy reasons, but my guess is in some cases
	// we do want to add a slug
	return defaultValidator.toValidName(s, nameMaxLength, false)
}

func ToValidCronjobName(s string) (string, error) {
	return defaultValidator.toValidName(s, cronjobNameMaxLength, true)
}

// Implementation that uses the singleton
// truncateWithSlug: maintains uniqueness by adding a deterministic slug which
// is hash of input string.
func (v *validator) toValidName(
	name string,
	maxLength int,
	truncateWithSlug bool,
) (string, error) {
	// Is valid name doesn't work from cronjobs. Add length check to ensure
	// cronjobs work as well.
	if IsValidName(name) && len(name) <= maxLength {
		return name, nil
	}
	s := ToIdentifier(name)
	s = strings.ToLower(s)

	// Allow only '-' as a separator
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.ReplaceAll(s, ".", "-")

	// Ensure the first character is alphabetic
	s = v.nonAlphabeticPrefix.ReplaceAllString(s, "")

	if len(s) == 0 {
		return "", ErrInvalidName
	}

	if len(s) > maxLength {
		if truncateWithSlug {
			suffix := "-" + DeterministicSlug(name)
			s = s[:(maxLength-len(suffix))] + suffix
		} else {
			// Truncate and clean suffixes again.
			s = ToIdentifier(s[:maxLength])
		}
	}

	return s, nil
}

// ToValidNameWithSlug adds a slug to the `prefix` and converts it to a valid name
func ToValidNameWithSlug(prefix string) (string, error) {
	return toValidNameWithSlug(prefix, KubeSlug)
}

// toValidNameWithSlug is an internal function that takes slugFn as a parameter
// so that it can be mocked out during testing.
func toValidNameWithSlug(prefix string, slugFn func() string) (string, error) {

	// minus 1 for the `-` added to the slug from slugFn
	const prefixMax = nameMaxLength - kubernetesSlugLength - 1
	if len(prefix) > prefixMax {
		prefix = prefix[:prefixMax]
	}

	// Uses the same logic found in k8s code to add a slug.
	// See https://github.com/kubernetes/apiserver/blob/master/pkg/storage/names/generate.go
	nameWithSlug := fmt.Sprintf("%s-%s", prefix, slugFn())
	name, err := ToValidName(nameWithSlug)
	if err != nil {
		return "", errors.WithStack(err)
	}
	return name, nil
}

// KubeSlug returns a slug of the length used in kubernetes code.
// This function is useful for mocking in tests (for stability).
func KubeSlug() string {
	return rand.String(kubernetesSlugLength)
}

func DeterministicSlug(seed string) string {
	hash := fnv.New32a().Sum([]byte(seed))
	// base 32 provides a bit more collision protection than hex. Avoid base 64
	// so we can make everything lowercase.
	slug := base32.StdEncoding.EncodeToString(hash)[:kubernetesSlugLength]
	return strings.ToLower(slug)
}
