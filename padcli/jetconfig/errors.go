package jetconfig

import "github.com/pkg/errors"

var ErrConfigNotFound = errors.New("jetconfig (launchpad.yaml) was not found")
var ErrInvalidProjectID = errors.New("project ID is invalid")
