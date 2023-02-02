package errorutil

import (
	"fmt"

	"github.com/pkg/errors"
)

// combinedError is a custom error that allows us to return an internal error and
// an additional error meant for the user. For example
//
// return combinedError(
//
//	originalError
//	errGCPNotAuthenticated, // created using NewUserError
//
// )
//
// in this case `errGCPNotAuthenticated` can have a user friendly error message
// while causeError contains the full error that caused to problem.
//
// combinedError will mostly behave like a normal error. If Error() is called, it
// Will wrap the original with the user error in order to provide the most context
// but it also allows us to use Is() with custom errors so we can show messages
// directly to user.
//
// example Is() usage: errors.Is(err, errGCPNotAuthenticated)
// returns true if the original or the combinedError is the target error.
//
// You can also leverage errors.As() like this:
//
// ue := errorutil.EmptyCombinedError()
//
//	if errors.As(err, &ue) {
//	  fmt.Println(ue.UserError())
//	}
type combinedError struct {
	original  error
	userError *userError
}

// userError makes CombinedError() less error-prone by requiring user to define
// which error is the user error.
type userError struct {
	error
}

type formatted interface {
	error
	Format(s fmt.State, verb rune)
}

func NewUserError(msg string) *userError {
	return &userError{error: errors.New(msg)}
}

func NewUserErrorf(msg string, args ...any) *userError {
	return &userError{error: errors.New(fmt.Sprintf(msg, args...))}
}

func CombinedError(original error, userErr *userError) error {
	if original == nil || hasUserError(original) {
		return original
	}
	return &combinedError{original, userErr}
}

func AddUserMessagef(original error, msg string, args ...any) error {
	if original == nil || hasUserError(original) {
		return original
	}
	return &combinedError{original, NewUserError(fmt.Sprintf(msg, args...))}
}

func ConvertToUserError(err error) error {
	if err == nil || hasUserError(err) {
		return err
	}
	return AddUserMessagef(err, err.Error())
}

func GetUserErrorMessage(err error) string {
	ce := &combinedError{}
	if errors.As(err, &ce) {
		return ce.UserError().Error()
	}
	us := &userError{}
	if errors.As(err, &us) {
		return us.Error()
	}
	return ""
}

func (err *combinedError) Error() string {
	return err.Combine().Error()
}

func (err *combinedError) UserError() error {
	return err.userError
}

func (err *combinedError) Combine() formatted {
	var f formatted // We don't need errors.As here, but it makes the linter happy
	errors.As(errors.Wrap(err.original, err.userError.Error()), &f)
	return f
}

// Is equals to either the cause or the user error
func (err *combinedError) Is(target error) bool {
	return errors.Is(err.original, target) || errors.Is(err.userError, target)
}

// Unwrap provides compatibility for Go 1.13 error chains.
func (err *combinedError) Unwrap() error { return err.Cause() }

// Leverage functionality of errors.Cause
func (err *combinedError) Cause() error { return errors.Cause(err.original) }

// Format allows us to use %+v as implemented by github.com/pkg/errors.
func (err *combinedError) Format(s fmt.State, verb rune) {
	err.Combine().Format(s, verb)
}

// hasUserError returns true if the error is a user error or combined
func hasUserError(err error) bool {
	ce := &combinedError{}
	us := &userError{}
	return errors.As(err, &ce) || errors.As(err, &us)
}
