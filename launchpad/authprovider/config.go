package authprovider

import (
	"os"

	dockerConfig "github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/configfile"
	"github.com/docker/cli/cli/config/types"
)

type config struct {
	*configfile.ConfigFile
	authConfigs map[string]types.AuthConfig
}

func NewConfig(host, username, password string) *config {
	return &config{
		ConfigFile: dockerConfig.LoadDefaultConfigFile(os.Stderr),
		authConfigs: map[string]types.AuthConfig{
			host: {
				Username: username,
				Password: password,
			},
		},
	}
}

func (c *config) GetAuthConfig(host string) (types.AuthConfig, error) {
	if a, ok := c.authConfigs[host]; ok {
		return a, nil
	}
	return c.ConfigFile.GetAuthConfig(host)
}
