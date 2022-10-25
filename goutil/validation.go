package goutil

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/pkg/errors"
)

func ValidateStructFieldsAreNotZero(s any, fields ...string) error {
	errorStrings := []string{}
	v := reflect.ValueOf(s).Elem()
	for _, f := range fields {
		if v.FieldByName(f).IsZero() {
			errorStrings = append(errorStrings, fmt.Sprintf("%s is missing", f))
		}
	}
	if len(errorStrings) == 0 {
		return nil
	}
	return errors.New(strings.Join(errorStrings, ", "))
}
