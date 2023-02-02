package errorutil

import (
	"testing"

	"github.com/pkg/errors"
)

func TestHasUserError(t *testing.T) {
	err := NewUserError("test")
	if !hasUserError(err) {
		t.Errorf("got HasUserError(%q) = false, want true.", err)
	}

	combined := CombinedError(errors.New("rand"), NewUserError("test"))
	if !hasUserError(combined) {
		t.Errorf("got HasUserError(%q) = false, want true.", combined)
	}

	notUserError := errors.New("no user error")
	if hasUserError(notUserError) {
		t.Errorf("got HasUserError(%q) = true, want false.", notUserError)
	}
}

func TestDontRewrapUserError(t *testing.T) {
	err1 := NewUserError("test")
	err2 := NewUserError("test")
	userErr := CombinedError(err1, err2)
	// nolint:errorlint
	if userErr != err1 {
		t.Errorf("got CombinedError(%q, %q) = %q, want %q.", err1, err2, userErr, err1)
	}

	userErr = AddUserMessagef(err1, "test")
	// nolint:errorlint
	if userErr != err1 {
		t.Errorf("got AddUserMessagef(%q) = %q, want %q.", err1, userErr, err1)
	}
}

func TestAddUserMessagef(t *testing.T) {
	err := errors.New("test")
	userMessage := "test-user"
	userErr := AddUserMessagef(err, userMessage)
	// nolint:errorlint
	if userErr == err {
		t.Errorf("got AddUserMessagef(%q) = %q, want not %q.", err, userErr, err)
	}
	if GetUserErrorMessage(userErr) != userMessage {
		t.Errorf(
			"got GetUserErrorMessage(%q) = %q, want %q.",
			userErr,
			GetUserErrorMessage(userErr),
			"test",
		)
	}
}
