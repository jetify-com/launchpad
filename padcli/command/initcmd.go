package command

import (
	"context"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.jetpack.io/launchpad/goutil/errorutil"
	"go.jetpack.io/launchpad/padcli/jetconfig"
	"go.jetpack.io/launchpad/padcli/provider"
	"go.jetpack.io/launchpad/pkg/jetlog"
	"go.jetpack.io/launchpad/pkg/kubevalidate"
)

func initCmd() *cobra.Command {
	var initCmd = &cobra.Command{
		Use:   "init [path]",
		Short: "Initializes a new Launchpad config",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			authProvider := cmdOpts.AuthProvider()
			path, err := absPath(args)
			if err != nil {
				return errors.WithStack(err)
			}
			return initConfig(cmd.Context(), authProvider, path)
		},
	}

	return initCmd
}

func absPath(args []string) (string, error) {
	relPath := "."
	if len(args) > 0 && args[0] != "" {
		relPath = args[0]
	}
	path, err := filepath.Abs(relPath)
	return path, errors.WithStack(err)
}

// projectDir returns the absolute path to the project directory.
// returns an error if the path is invalid. If path is a file (jetconfig) we
// return the directory.
func projectDir(args []string) (string, error) {
	path, err := absPath(args)
	if err != nil {
		return "", errors.WithStack(err)
	}

	dir := jetconfig.ConfigDir(path)
	fi, err := os.Stat(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", errorutil.NewUserErrorf("The path \"%s\" doesn't exist.", dir)
		}
		return "", errorutil.NewUserErrorf("Error reading path \"%s\": %v", dir, err)
	}
	if !fi.IsDir() {
		// This is unexpected because fi should always be a directory.
		return "", errUnexpectedFile
	}
	return dir, nil
}

func initConfig(ctx context.Context, authProvider provider.Auth, path string) error {
	appName, err := appName(path)
	if err != nil {
		return errors.WithStack(err)
	}
	// check if jetconfig file exists
	_, err = jetconfig.RequireFromFileSystem(ctx, path, cmdOpts.RootFlags().Env())
	if err == nil {
		jetlog.Logger(ctx).Printf(
			"%s already exists. Please edit directly",
			path,
		)
		return nil
	} else if !errors.Is(err, jetconfig.ErrConfigNotFound) {
		return errors.WithStack(err)
	}

	answers, err := cmdOpts.InitSurveyProvider().Run(ctx, authProvider, appName)
	if err != nil {
		return errors.WithStack(err)
	}
	if answers == nil {
		return nil
	}

	if answers.ClusterOption == provider.CreateJetpackCluster {
		// Ask users to create a cluster first.
		jetlog.Logger(ctx).Printf("\nTo create a new cluster, run `launchpad cluster create <cluster_name>`.\n")
		return nil
	}

	jetCfg := &jetconfig.Config{
		ConfigVersion: jetconfig.Versions.Prod(),
		Name:          answers.AppName,
		Services:      []jetconfig.Service{},
	}

	if answers.AppType == string(provider.WebServiceType) {
		jetCfg.AddNewWebService(answers.AppName + "-" + jetconfig.WebType)
	} else if answers.AppType == string(provider.CronjobServiceType) {
		jetCfg.AddNewCronService(
			answers.AppName+"-"+jetconfig.CronType,
			[]string{"/bin/sh", "-c", "date; echo Hello from Launchpad"},
			"* * * * *",
		)
	}

	if answers.ClusterOption != "" && answers.ClusterOption != provider.CreateJetpackCluster {
		jetCfg.Cluster = answers.ClusterOption
	}
	if answers.ImageRepositoryLocation != "" {
		jetCfg.ImageRepository = answers.ImageRepositoryLocation
	}

	jetCfg.ConfigVersion = jetconfig.Versions.Prod()
	jetCfg.ProjectID = jetconfig.NewProjectId()

	configPath, err := jetCfg.SaveConfig(path)
	if err != nil {
		return errors.WithStack(err)
	}
	jetlog.Logger(ctx).Printf(
		"\nWritten config file at %s. Be sure to add it to your git "+
			"repository.\nFor reference guide, visit: "+
			"https://www.jetpack.io/launchpad/docs/reference/launchpad.yaml-reference/ \n",
		configPath,
	)

	return nil
}

func appName(path string) (string, error) {
	return kubevalidate.ToValidName(filepath.Base(jetconfig.ConfigDir(path)))
}
