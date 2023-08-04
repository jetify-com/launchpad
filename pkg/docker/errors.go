package docker

import "go.jetpack.io/launchpad/goutil/errorutil"

var errOldDockerAPIVersion = errorutil.NewUserError(
	"Launchpad requires your Docker API version to be at least 1.39",
)
