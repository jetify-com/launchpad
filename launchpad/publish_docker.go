package launchpad

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/configfile"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/homedir"
	"github.com/pkg/errors"
	"go.jetpack.io/launchpad/goutil/errorutil"
)

// Gets the base64 encoded RegistryAuth parameter that dockerclient.ImagePush
// will use. Uses the docker credential store on the user's laptop to find
// the credentials. It requires the docker credential store to have been
// configured already.
//
// inspired by:
// https://github.com/GoogleContainerTools/skaffold/blob/main/pkg/skaffold/docker/auth.go
func credentialsFromDockerCredentialStore(registryHostname string) (string, error) {

	cf, err := loadDockerConfig()
	if err != nil {
		return "", errors.Wrap(err, "error loading docker config")
	}

	auth, err := cf.GetAuthConfig(registryHostname)
	if err != nil {
		return "", errorutil.CombinedError(
			errors.Wrapf(err, "error getting auth config for registry: %s", registryHostname),
			errUserNoGCPCredentials,
		)
	}

	ac := types.AuthConfig(auth)
	jsonAuthConfig, err := json.Marshal(ac)
	if err != nil {
		return "", errors.Wrap(err, "failed to json marshal")
	}

	return base64.URLEncoding.EncodeToString(jsonAuthConfig), nil
}

// Gets the Docker ConfigFile
//
// inspired by:
// https://github.com/GoogleContainerTools/skaffold/blob/main/pkg/skaffold/docker/auth.go
func loadDockerConfig() (*configfile.ConfigFile, error) {

	configDir := os.Getenv("DOCKER_CONFIG")
	const configFileDir = ".docker"
	if configDir == "" {
		configDir = filepath.Join(homedir.Get(), configFileDir)
	}

	cf, err := config.Load(configDir)
	if err != nil {
		return nil, errors.Wrapf(err, "failed loading docker config")
	}
	return cf, nil
}
