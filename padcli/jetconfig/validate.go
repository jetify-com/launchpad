package jetconfig

import (
	"strings"

	"github.com/pkg/errors"
	"go.jetpack.io/launchpad/goutil/errorutil"
	"go.jetpack.io/launchpad/padcli/semver"
	"go.jetpack.io/launchpad/proto/api"
)

var atMostOneWebServiceErr = validationError("At most one web service may be defined per jetconfig")

func (cfg *Config) validate() error {
	checkers := []func(cfg *Config) error{
		requireConfigVersionRule,
		validConfigVersionRule,
		requireNameRule,
		validNameRule,
		requireProjectIdRule,
		validProjectIdRule,
		requireClusterRule,
		atMostOneWebServiceRule,
		validateSelectedEnvironmentRule,
	}
	for _, checker := range checkers {
		if err := checker(cfg); err != nil {
			return errors.WithStack(err)
		}
	}
	return nil
}

func requireConfigVersionRule(cfg *Config) error {
	if cfg.ConfigVersion == "" {
		return validationError("ConfigVersion is required")
	}
	return nil
}

func validConfigVersionRule(cfg *Config) error {
	// Keeping this for backwards-compat, so as to not break existing examples.
	vers := []string{legacyVersionOneDotZero, legacyVersionOneDotOne}
	for _, v := range vers {
		if cfg.ConfigVersion == v {
			return nil
		}
	}

	if semver.IsValid(cfg.ConfigVersion) {
		return nil
	}

	return validationError("ConfigVersion %s should be one of %v", cfg.ConfigVersion, vers)
}

func requireNameRule(cfg *Config) error {
	if cfg.Name == "" {
		return validationError("Name is required")
	}
	return nil
}

func validNameRule(cfg *Config) error {
	const minNameLength = 4
	if cfg.Name != "" && len(cfg.Name) < minNameLength {
		return validationError("Name must be at least %d characters long", minNameLength)
	}
	return nil
}

func requireProjectIdRule(cfg *Config) error {
	if cfg.ProjectID == "" {
		return validationError("ProjectID is required")
	}
	return nil
}

func validProjectIdRule(cfg *Config) error {
	if !strings.HasPrefix(cfg.ProjectID, "proj_") {
		return validationError("ProjectID must start with proj_")
	}
	// We can add more checks here, like base52decode and check if valid uuid

	return nil
}

func requireClusterRule(cfg *Config) error {
	if cfg.ConfigVersion != Versions.Prod() {
		return nil
	}

	if cfg.Cluster == "" {
		return validationError("Cluster is required. Run \"jetpack cluster ls\" to see a list of clusters available to you. Then add \"cluster: <cluster-name>\" to your jetconfig.")
	}
	return nil
}

func atMostOneWebServiceRule(cfg *Config) error {
	if len(cfg.Services) == 0 {
		return nil
	}

	webCount := 0
	for _, svc := range cfg.Services {
		if _, ok := svc.(*web); ok {
			webCount += 1
		}
	}
	if webCount > 1 {
		return atMostOneWebServiceErr
	}
	return nil
}

func validationError(msg ...any) error {
	return errorutil.NewUserErrorf("Invalid Jetconfig Error: %s", msg...)
}

func validateSelectedEnvironmentRule(cfg *Config) error {
	if cfg.selectedEnvironment == api.Environment_NONE {
		return validationError("Environment cannot be NONE")
	}
	return nil
}
