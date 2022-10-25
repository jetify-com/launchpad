package semver

import (
	"errors"
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

func (s *Suite) TestCompare() {
	req := s.Require()

	cases := []struct {
		in1  string
		in2  string
		want int
		err  error // should it error?
	}{
		// first value < second value
		{"1", "2", -1, nil},
		{"0.0.1", "0.0.2", -1, nil},
		{"1.2.3", "2.3.4", -1, nil},

		// equal values
		{"1", "1", 0, nil},
		{"1.0", "1.0", 0, nil},
		{"1.0.0", "1.0.0", 0, nil},
		{"0.0.1", "0.0.1", 0, nil},

		// first value > second value
		{"2", "1", 1, nil},
		{"1.2.3", "0.1.2", 1, nil},

		// invalid values
		{"v1", "v1", 0, errInvalidValue},
		{"-1", "1.2.3", 0, errInvalidValue},
		{"1.2.3", "-1", 0, errInvalidValue},
		{"1.2.3.4", "1.2.3", 0, errInvalidValue},
	}

	for _, tc := range cases {
		s.T().Run(fmt.Sprintf("compare_%s_%s", tc.in1, tc.in2), func(t *testing.T) {
			got, err := Compare(tc.in1, tc.in2)
			if tc.err == nil && err != nil {
				req.Fail("received unexpected error", "error: %v", err)
			}
			if tc.err != nil && err == nil {
				req.Fail("expected error but didn't receive one", "expected: %v", err)
			}
			if tc.err != nil && err != nil && !errors.Is(err, tc.err) {
				req.Failf("received error is not the expected one", "expected error received %v to be a %v",
					err.Error(),
					tc.err.Error())
			}
			if got != tc.want {
				req.Fail("expected %d but got %d", tc.want, got)
			}
		})
	}
}

func (s *Suite) TestIsValid() {
	req := s.Require()

	cases := []struct {
		in   string
		want bool
	}{
		// invalid values
		{"v1", false},
		{"-1", false},
		{"1.2.3.4", false},

		// valid values
		{"1", true},
		{"1.0", true},
		{"1.0.0", true},
		{"1.2.3", true},
		{"3.2.1", true},
		{"3.2", true},
	}

	for _, tc := range cases {
		s.T().Run(fmt.Sprintf("is_valid_%s", tc.in), func(t *testing.T) {
			got := IsValid(tc.in)
			if got != tc.want {
				req.Fail("expected %d but got %d", tc.want, got)
			}
		})
	}
}
