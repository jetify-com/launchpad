package jetconfig

import (
	_ "embed"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"
)

//go:embed jetconfig_test_v0_1_2.yaml
var jetconfigYaml_v0_1_2 string

//go:embed jetconfig_test_multi_url.yaml
var jetconfig_test_multi_url string

type Suite struct {
	suite.Suite
}

func TestSuite(t *testing.T) {
	suite.Run(t, &Suite{})
}

func (s *Suite) TestLoad() {
	cases := []struct {
		version      string
		yamlContents string
	}{
		{"0.1.2", jetconfigYaml_v0_1_2},
		// Made new config because we don't support 2 web services in the same config
		{"0.1.2-multi-url", jetconfig_test_multi_url},
	}

	for _, tc := range cases {
		s.T().Run(fmt.Sprintf("version:%s", tc.version), func(t *testing.T) {
			loadConfigOfVersion(t, tc.yamlContents)
		})
	}
}

func loadConfigOfVersion(t *testing.T, yamlContents string) {
	req := require.New(t)
	cfg := &Config{}
	err := cfg.loadConfigFromYamlContents([]byte(yamlContents))
	req.NoError(err)

	crons := cfg.Cronjobs()
	req.Equal(2, len(crons))
	for _, cron := range crons {
		if cron.GetName() == "date-printer-cron" {
			req.Equal(cron.GetImage(), "busybox:latest")
		} else if cron.GetName() == "ls-cron" {
			req.Equal(cron.GetImage(), "")
		} else {
			req.Fail("unexpected name", "name %s", cron.GetName())
		}
	}

	svc, err := cfg.WebService()
	req.NoError(err)
	if svc.GetName() == "ghost" {
		req.Equal(svc.GetImage(), "ghost:4.26.1-alpine")
	} else {
		req.Fail("unexpected name", "name %s", svc.GetName())
	}
}

func (s *Suite) TestSave() {

	cases := []struct {
		version      string
		yamlContents string
	}{
		{"0.1.2", jetconfigYaml_v0_1_2},
	}

	for _, tc := range cases {
		s.T().Run(fmt.Sprintf("version:%s", tc.version), func(t *testing.T) {
			saveConfigOfVersion(t, tc.yamlContents)
		})
	}
}

func saveConfigOfVersion(t *testing.T, yamlContents string) {
	req := require.New(t)

	cfg := &Config{}
	err := cfg.loadConfigFromYamlContents([]byte(yamlContents))
	req.NoError(err)

	// This marshalYaml simulates what would be saved in a file.
	marshalled, err := cfg.marshalYaml()
	req.NoError(err)

	// we load the saved-jetconfig content into a map
	var mapTest map[string]any
	err = yaml.Unmarshal(marshalled, &mapTest)
	req.NoError(err)

	// we load the original jetconfig content into a map
	var mapControl map[string]any
	err = yaml.Unmarshal([]byte(yamlContents), &mapControl)
	req.NoError(err)

	// finally, we compare the original versus saved jetconfig contents
	req.Equal(mapControl, mapTest)
}
