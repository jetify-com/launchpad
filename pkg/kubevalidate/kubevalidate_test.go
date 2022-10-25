package kubevalidate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/validation"
)

func TestToIdentifier(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		// Prefix examples:
		{".foo", "foo"},
		{"   ###  0foo", "0foo"},
		// Suffix examples:
		{"foo...", "foo"},
		{"foo0 $$ ", "foo0"},
		// Separator examples:
		{"Daniel's car", "Daniels-car"},
		{"name::.other", "name.other"},
		{"foo---___...bar", "foo-bar"},
		{"foo.-.-.-.-bar", "foo.bar"},
		{"foo-.-.-.bar", "foo-bar"},
		{"foo!@#!@#@!#_____111bar", "foo_111bar"},
		// Combined examples:
		{" FOObar?Foo.", "FOObar-Foo"},
		{"    foo  ::   bar     ::", "foo-bar"},
		// No valid characters examples:
		{"", ""},
		{"______", ""},
		{".-___-.", ""},
	}

	for _, testCase := range testCases {
		t.Run(testCase.input, func(t *testing.T) { // Use input as test case name
			output := ToIdentifier(testCase.input)
			assert.Equal(t, testCase.expected, output)
		})
	}
}

func TestToValidSubdomain(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{" FOO___bar?.Foo.", "foo-bar.foo"},
		{"    foo  ::   bar     ::", "foo-bar"},
		{"##0f0", "0f0"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.input, func(t *testing.T) { // Use input as test case name
			output, err := ToValidSubdomain(testCase.input)

			errs := validation.IsDNS1123Subdomain(output)
			assert.True(t, len(errs) == 0, "%s is not a valid subdomain %v+", output, errs)

			assert.NoError(t, err)
			assert.Equal(t, testCase.expected, output)
		})
	}

	// These should all cause an error
	errorTestCases := []string{
		"",
		"______",
	}

	for _, input := range errorTestCases {
		t.Run(input, func(t *testing.T) {
			output, err := ToValidSubdomain(input)
			assert.Empty(t, output)
			assert.Error(t, err)
		})
	}
}

func TestToValidName(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{" FOO___bar?.Foo.", "foo-bar-foo"},
		{"    foo  ::   bar     ::", "foo-bar"},
		{"##0f0", "f0"},
		{
			"thisisalongstringthatisover63charactersandshouldbetruncatedhereextra",
			"thisisalongstringthatisover63charactersandshouldbetruncatedhere",
		},
		{
			"thisisalongstringthatisover63charactersand-should-be-truncated-extra",
			"thisisalongstringthatisover63charactersand-should-be-truncated",
		},
		{"05-py-hello-dockerfile", "py-hello-dockerfile"},
		{"name.with.periods", "name-with-periods"},

		// These are the test-cases from the (internal) pkg/docker
		{"py-hello-dashes", "py-hello-dashes"},
		{"py_hello_underscores", "py-hello-underscores"},
		{"py-HELLO-Multicase", "py-hello-multicase"},
		{"PY-HELLO-UPPERCASE", "py-hello-uppercase"},
		{"py-$pecia|-ch@r", "py-pecia-ch-r"},
		{" spaces-around ", "spaces-around"},
		{"spaces  within", "spaces-within"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.input, func(t *testing.T) { // Use input as test case name
			output, err := ToValidName(testCase.input)

			errs := validation.IsDNS1035Label(output)
			assert.True(t, len(errs) == 0, "%s is not a valid label %v+", output, errs)

			assert.NoError(t, err)
			assert.Equal(t, testCase.expected, output)
		})
	}

	// These should all cause an error
	errorTestCases := []string{
		"",
		"______",
	}

	for _, input := range errorTestCases {
		t.Run(input, func(t *testing.T) {
			output, err := ToValidSubdomain(input)
			assert.Empty(t, output)
			assert.Error(t, err)
		})
	}
}

func TestToValidNameWithSlug(t *testing.T) {

	randoSlug := KubeSlug()
	testCases := []struct {
		input    string
		expected string
	}{
		{"py-hello-world", "py-hello-world-" + randoSlug},
		{
			"thisisalongstringthatisover63charactersandshouldbetruncatedhereextra",
			"thisisalongstringthatisover63charactersandshouldbetruncat-" + randoSlug,
		},
	}

	slugFn := func() string {
		return randoSlug
	}
	for _, testCase := range testCases {
		t.Run(testCase.input, func(t *testing.T) {
			output, err := toValidNameWithSlug(testCase.input, slugFn)

			require.NoError(t, err)
			require.Equal(t, testCase.expected, output)
		})
	}
}

func TestToValidCronjobName(t *testing.T) {

	testCases := []struct {
		input    string
		expected string
	}{
		{"py-hello-world", "py-hello-world"},
		{
			"thisisalongstringthatisover52charactersandshouldbetruncatedhereextra",
			"thisisalongstringthatisover52charactersandshou-orugs",
		},
		{
			// 54 char long (longer than 52 shorter than 63)
			"a23456789012345678901234567890123456789012345678901234",
			"a234567890123456789012345678901234567890123456-mezdg",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.input, func(t *testing.T) {
			o, err := ToValidCronjobName(testCase.input)
			require.NoError(t, err)
			require.Equal(t, testCase.expected, o)
		})
	}
}
