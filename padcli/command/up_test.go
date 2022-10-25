package command

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/spf13/cobra"
	"go.jetpack.io/launchpad/launchpad"
	"go.jetpack.io/launchpad/padcli/command/mock"
	"go.jetpack.io/launchpad/padcli/flags"
	"go.jetpack.io/launchpad/padcli/helm"
	"go.jetpack.io/launchpad/padcli/jetconfig"
	"go.jetpack.io/launchpad/proto/api"
)

// Testing docker build publish and deploy for local registry
// Since it is not pushing to ECR or GCP, the publish step is essentially skipped.
// This is a baseline test that skips a ton of logic which needs individual unit tests.
// Note: the docker build step can be SLOW. Ideally this step should not be re-run per test.
// Recommend mocking pad.build for other tests.
func (t *Suite) TestBuildPublishAndDeploy() {
	req := t.Require()
	ctx := context.Background()
	cmdOpts = &mock.MockCmdOptions{
		RootCMDFlags: &flags.RootCmdFlags{},
	}

	cmd := upCmd()

	projectName := "py-dockerfile"
	projectID := "proj_4pss8bskaTPOWzuhyY7cfL"

	jetCfg := &jetconfig.Config{
		ConfigVersion:   jetconfig.Versions.Prod(),
		ProjectID:       projectID,
		Name:            projectName,
		ImageRepository: "test-repository",
		Cluster:         "jetpack-trial-context",
		Environment: map[string]jetconfig.EnvironmentFields{
			"dev": {},
		},
		Services: []jetconfig.Service{},
	}

	jetCfg.AddNewWebService("py-dockerweb")
	pad := launchpad.NewPad(cmdOpts.ErrorLogger())
	expectedDeployOptions := &launchpad.DeployOptions{
		App: &launchpad.HelmOptions{
			ReleaseName:  "proj-4pss8bskatpowzuhyy7cfl",
			InstanceName: "py-dockerfile-py-dockerweb",
			Values: map[string]any{
				"jetpack": map[string]any{},
			},
		},
		KubeContext: jetCfg.Cluster,
		Namespace:   "test-ns",
	}
	testPad := &mock.MockPad{
		BuildFunc:   pad.Build, // Skip build function here by mocking
		PublishFunc: pad.Publish,
		DeployFunc: func(ctx context.Context, opts *launchpad.DeployOptions) (*launchpad.DeployOutput, error) {

			req.Equal(expectedDeployOptions.App.ReleaseName, opts.App.ReleaseName)
			req.Equal(expectedDeployOptions.App.InstanceName, opts.App.InstanceName)

			msg := fmt.Sprintf(
				"deployOptions.App.Values.autoscaling. received: %v, expect: %v\n",
				opts.App.Values["autoscaling"],
				expectedDeployOptions.App.Values["autoscaling"],
			)
			req.True(
				reflect.DeepEqual(opts.App.Values["autoscaling"], expectedDeployOptions.App.Values["autoscaling"]),
				msg,
			)
			req.Equal(expectedDeployOptions.Namespace, opts.Namespace)

			return &launchpad.DeployOutput{}, nil
		},
		PortForwardFunc: pad.PortForward,
	}

	path := t.T().TempDir()
	configPath, err := jetCfg.SaveConfig(path)
	req.NoError(err)

	cmdOpts.RootFlags().Environment = "dev"
	jetCfg, err = RequireConfigFromFileSystem(ctx, cmd, []string{configPath})
	req.NoError(err)

	// Copy the files under testdata to the temp directory
	// And simulate the docker build process in that directory
	err = copyFile(path, "Dockerfile")
	req.NoError(err)
	err = copyFile(path, "requirements.txt")
	req.NoError(err)
	err = copyFile(path, "jetpack_main.py")
	req.NoError(err)

	// We do not copy the .env file for now, as the unit tests on the deploy function
	// is preferred for testing env logic.

	opts := upOpts
	do := &opts.deployOptions
	do.Namespace = expectedDeployOptions.Namespace
	bo := &opts.embeddedBuildOptions
	cluster := mock.NewClusterForTest(jetCfg.Cluster, true)

	repoConfig, err := cmdOpts.RepositoryProvider().Get(ctx, cluster)
	req.NoError(err)

	_, err = buildPublishAndDeploy(
		ctx,
		testPad,
		cmd,
		jetCfg,
		bo,
		do,
		"", // No override means no publishing
		path,
		cluster,
		repoConfig,
		mock.NewEnvsecStore(),
	)
	req.NoError(err)
}

// Test for deploying with .env file present, and make sure
// the env values are piped to helm
func (t *Suite) TestDeployWithDotEnvFile() {
	req := t.Require()
	ctx := context.Background()

	cmd := upCmd()

	jetCfg := &jetconfig.Config{
		ConfigVersion: "1.0",
		ProjectID:     "proj_4pss8BskaTPOWzuhyY7cfL",
		Name:          "py-dockerfile",
		Cluster:       "jetpack-trial-context",
		Environment: map[string]jetconfig.EnvironmentFields{
			"dev": {},
		},
	}

	path := t.T().TempDir()
	_, err := jetCfg.SaveConfig(path)
	req.NoError(err)

	// Copy the env file under testdata to the temp directory
	err = copyFile(path, ".env")
	req.NoError(err)

	do := &deployOptions{}
	do.envOverrideFile = ".env"
	do.Namespace = "test-ns"
	po := &launchpad.PublishOutput{}
	bo := &launchpad.BuildOutput{}

	cluster := mock.NewClusterForTest(jetCfg.Cluster, true)

	cmdOpts.RootFlags().Environment = "dev"

	_, err = makeDeployOptions(
		ctx,
		cmd,
		jetCfg,
		po,
		bo,
		do,
		path,
		cluster,
		mock.NewEnvsecStore(),
	)
	req.NoError(err)
}

func copyFile(dest string, filename string) error {
	file, err := os.ReadFile(filepath.Join("testdata", filename))
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dest, filename), file, 0666)
}

func (t *Suite) TestComputeHelmValues() {
	req := t.Require()
	clusterHostname := "cluster.jetpack.dev"
	cmdOpts.RootFlags().Environment = "dev"

	projectName := "py-dockerfile"
	projectID := "proj_4pss8BskaTPOWzuhyY7cfL"
	cronJetCfg := &jetconfig.Config{
		ProjectID: projectID,
		Name:      projectName,
		Environment: map[string]jetconfig.EnvironmentFields{
			"dev": {},
		},
		Services: []jetconfig.Service{},
	}
	cronJetCfg.AddNewCronService(
		"data-cron",
		[]string{"/bin/sh", "-c", "date; echo Hello from Jetpack"},
		"* * * * *",
	)

	cases := []struct {
		name         string
		cmd          *cobra.Command
		opts         *deployOptions
		jetCfg       *jetconfig.Config
		verifyResult func(do *deployOptions, appValues map[string]any, runtimeValues map[string]any)
	}{
		{
			"single",
			upCmd(),
			&deployOptions{
				execQualifiedSymbol: "cron.my_cron",
			},
			&jetconfig.Config{
				ProjectID: projectID,
				Name:      "py-dockerfile",
				Environment: map[string]jetconfig.EnvironmentFields{
					"dev": {},
				},
			},
			func(do *deployOptions, appValues map[string]any, runtimeValues map[string]any) {
				req.True(appValues["jetpack"].(map[string]any)["projectId"] == "proj_4pss8BskaTPOWzuhyY7cfL")
				req.True(appValues["jetpack"].(map[string]any)["runSDKExec"] == true)
				req.True(appValues["jetpack"].(map[string]any)["qualifiedSymbol"] == "cron.my_cron")
				req.True(appValues["jetpack"].(map[string]any)["runSDKRegister"] == false)
				req.True(appValues["jetpack"].(map[string]any)["clusterHostname"] == clusterHostname)
			},
		},
		{
			"cronjobs",
			upCmd(),
			&deployOptions{},
			cronJetCfg,
			func(do *deployOptions, appValues map[string]any, runtimeValues map[string]any) {
				req.Equal("proj_4pss8BskaTPOWzuhyY7cfL", appValues["jetpack"].(map[string]any)["projectId"])
				req.Nil(appValues["jetpack"].(map[string]any)["runSDKExec"])
				req.Nil(appValues["jetpack"].(map[string]any)["qualifiedSymbol"])
				req.Nil(appValues["jetpack"].(map[string]any)["runSDKRegister"])
				req.Equal(clusterHostname, appValues["jetpack"].(map[string]any)["clusterHostname"])
				req.Equal("py-dockerfile-data-cron", appValues["jetpack"].(map[string]any)["cronjobs"].([]interface{})[0].(map[string]any)["name"])
			},
		},
	}

	for _, tc := range cases {
		t.T().Run(fmt.Sprintf("%v", tc.name), func(t *testing.T) {
			err := addFlagsToCmd(tc.cmd, []string{}, tc.jetCfg)
			req.NoError(err)
			ctx := context.Background()

			cluster := mock.NewJetpackManagedClusterForTest(
				tc.jetCfg.Cluster,
				clusterHostname,
			)
			hvc := helm.NewValueComputer(
				api.Environment_DEV,
				tc.opts.Namespace,
				tc.opts.execQualifiedSymbol,
				nil, // published images
				tc.jetCfg,
				cluster,
			)

			req.NoError(hvc.Compute(ctx))
			tc.verifyResult(tc.opts, hvc.AppValues(), hvc.RuntimeValues())
		})
	}
}
