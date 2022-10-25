package jetconfig

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"go.jetpack.io/launchpad/proto/api"
)

type ValidateSuite struct {
	suite.Suite
}

func TestValidateSuite(t *testing.T) {
	suite.Run(t, &ValidateSuite{})
}

func (s *ValidateSuite) TestValidate() {
	req := s.Require()

	cfg := Config{
		ConfigVersion: Versions.Prod(),
		Name:          "MyApp",
		Cluster:       "my-cluster",
		ProjectID:     "proj_1231231",
		Services:      []Service{},
	}
	cfg.AddNewWebService("my-first-web-service")
	cfg.selectedEnvironment = api.Environment_DEV

	err := cfg.validate()
	req.NoError(err)

	cfg.AddNewWebService("my-second-web-service")
	err = cfg.validate()
	req.Error(err)
	req.Equal(atMostOneWebServiceErr.Error(), err.Error())

	//req.True(false)
}
