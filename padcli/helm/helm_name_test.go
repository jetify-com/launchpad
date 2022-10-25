package helm

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"
)

type Suite struct {
	suite.Suite
}

func TestSuite(t *testing.T) {
	suite.Run(t, &Suite{})
}

func (s *Suite) TestValidName() {

	req := s.Require()

	cases := []struct {
		in   string
		want string
	}{
		// all lower case
		{"pyweb", "pyweb"},

		// all uppercase
		{"PYWEB", "pyweb"},

		// with camelCase
		{"pyWeb", "pyweb"},
		{"PyWeb", "pyweb"},

		// with hyphens
		{"py-web", "py-web"},
		{"py-Web", "py-web"},
		{"-py-web", "-py-web"}, // is this valid?
		{"py-web-", "py-web-"}, // is this valid?

		// with a digit:
		{"1pyweb", "1pyweb"},
		{"py1web", "py1web"},
		{"pyweb1", "pyweb1"},
		{"py1Web", "py1web"},
		{"pyw3b", "pyw3b"},

		// with special chars
		{"jetp@ck", "jetp-ck"},
		{"@jetpack", "-jetpack"},
		{"jetpack@", "jetpack-"},
		{"jetp@A@ck", "jetp-a-ck"},

		// these are examples of real projectIDs and project slugs
		{"p4pss8Bs-data-cron", "p4pss8bs-data-cron"},
		{"proj_4pss8bskaTPOWzuhyY7cfL", "proj-4pss8bskatpowzuhyy7cfl"},
	}

	for _, tc := range cases {
		s.T().Run(fmt.Sprintf("input:%s", tc.in), func(t *testing.T) {
			got := ToValidName(tc.in)
			if got != tc.want {
				req.Fail("Unexpected value", "expected %s but got %s", tc.want, got)
			}
		})
	}
}
