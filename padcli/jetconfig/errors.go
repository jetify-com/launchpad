package jetconfig

import "github.com/pkg/errors"

var ErrConfigNotFound = errors.New("jetconfig was not found")
var ErrInvalidProjectID = errors.New("project ID is invalid")
