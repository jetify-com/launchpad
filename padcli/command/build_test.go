package command

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.jetpack.io/launchpad/padcli/command/mock"
	"go.jetpack.io/launchpad/padcli/jetconfig"
)

type Suite struct {
	suite.Suite
}

func TestSuite(t *testing.T) {
	suite.Run(t, &Suite{})
}

func (t *Suite) TestProjectDir() {
	cwd, err := os.Getwd()
	if err != nil {
		t.T().Fatal("Error getting working directory:", err)
	}

	cases := []struct {
		in   string
		want string
		err  bool
	}{
		{"", cwd, false},
		{".", cwd, false},
		{"/", "/", false},
		{"./this/does/not/exist", "", true},
		{"/this/does/not/exist", "", true},
	}
	for _, tc := range cases {
		t.T().Run(fmt.Sprintf("%v", tc.in), func(t *testing.T) {
			got, err := projectDir([]string{tc.in})
			if tc.err && err == nil {
				t.Error("Got nil error.")
			}
			if !tc.err && err != nil {
				t.Error("Got error:", err)
			}
			if got != tc.want {
				t.Errorf("Got module path %q, want %q.", got, tc.want)
			}
		})
	}
}

func (t *Suite) TestMakeBuildOptions() {
	req := t.Require()
	ctx := context.Background()

	jetCfg := &jetconfig.Config{
		Name: "py-dockerfile",
		Environment: map[string]jetconfig.EnvironmentFields{
			"dev": {},
		},
	}

	path := t.T().TempDir()

	opts := &buildOptions{}
	cluster := mock.NewClusterForTest("test-cluster", true)
	bo, err := makeBuildOptions(ctx, &opts.embeddedBuildOptions, jetCfg, cluster,
		path, nil /*repoConfig*/)

	req.NoError(err)
	req.True(bo.AppName == jetCfg.GetProjectName())
	req.True(bo.ProjectDir == path)
	req.NotNil(bo.Platform)
}

func (t *Suite) TestMakeBuildUnsupportedConfigFlag() {
	jetCfg := &jetconfig.Config{
		Name: "py-dockerfile",
		Environment: map[string]jetconfig.EnvironmentFields{
			"dev": {},
		},
	}

	path := t.T().TempDir()
	_, err := jetCfg.SaveConfig(path)
	t.Require().NoError(err)

	newJetCfg := &jetconfig.Config{
		Name: "py-dockerfile",
	} // new struct to ensure we load from the temporary file
	opts := &buildOptions{}
	cluster := mock.NewClusterForTest("test-cluster", true)
	_, err = makeBuildOptions(
		context.Background(),
		&opts.embeddedBuildOptions,
		newJetCfg,
		cluster,
		path,
		nil, // repoConfig
	)
	t.Require().NoError(err)
}
