package launchpad

import (
	"github.com/pkg/errors"
	"go.jetpack.io/launchpad/goutil/errorutil"
)

// errors during publish step
var errAwsConfigIsNilForUserSpecifiedRegistry = errors.New(
	"registry.awsCfg is nil even though the registry is NOT a jetpack provided one")

// errors during deploy step
var errInvalidChartConfig = errors.New("invalid chart config")
var errNoDeployRelease = errors.New("no release found")
var ErrPodContainerError = errorutil.NewUserError("deployment failed because of container error")

var errWaitForPodTimeout = errors.New("Timeout while waiting for pod to be ready")

var MsgUsingProdTrialClusterWhenLoggedOut = "Your kubeconfig is using the trial cluster but you are not logged" +
	"-in. Please do `launchpad auth login` and try again."

// User errors. Should start with errUser. Message will be visible to end user
var errUserNoGCPCredentials = errorutil.NewUserError(
	"Could not find GCP credentials. Did you forget to run `gcloud auth login`?",
)

var errInvalidFile = errorutil.NewUserError(
	"Could not load file",
)

var errUserNoDockerClient = errorutil.NewUserError(
	"Unable to get docker cli client. Are you sure Docker is installed?",
)

var errNoValidChartVersions = errors.New(
	"Could not find any valid chart versions",
)

var errUnableToAccessHelmReleases = errorutil.NewUserError(
	"Unable to access helm releases. You may not have permission to access the cluster or namespace. Please refresh your credentials or do `launchpad auth login` and try again.",
)

var errUserReinstallFail = errorutil.NewUserError(
	"Could not complete uninstall step of --reinstall-on-error. " +
		"Please use helm CLI tool to manually uninstall.",
)

var errUserUpgradeFail = errorutil.NewUserError(
	"Error upgrading chart. It is possible that existing release is corrupted. " +
		"Please use --reinstall-on-error to reinstall the release. " +
		"Warning: This may cause downtime.\n",
)
