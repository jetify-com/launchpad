package command

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.jetpack.io/launchpad/goutil/errorutil"
	"go.jetpack.io/launchpad/padcli/jetconfig"
	"go.jetpack.io/launchpad/padcli/provider"
	"go.jetpack.io/launchpad/pkg/jetlog"
	"go.jetpack.io/launchpad/pkg/kubevalidate"
)

type serviceTypeOption string

const (
	webServiceType        serviceTypeOption = "Web Service"
	cronjobServiceType    serviceTypeOption = "Cron Job"
	jetpackManagedCluster string            = "Jetpack managed cluster"
	createJetpackCluster  string            = "Create a new cluster with Jetpack"
)

type SurveyAnswers struct {
	AppName                 string
	AppType                 string
	ClusterOption           string
	KubeContext             string
	ImageRepositoryLocation string
}

func initCmd() *cobra.Command {
	var initCmd = &cobra.Command{
		Use:   "init [path]",
		Short: "init a new Launchpad config",
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

	answers, err := runConfigSurvey(ctx, authProvider, appName)
	if err != nil {
		return errors.WithStack(err)
	}
	if answers == nil {
		return nil
	}

	if answers.ClusterOption == createJetpackCluster {
		// Ask users to create a cluster first.
		jetlog.Logger(ctx).Printf("\nTo create a new cluster, run `launchpad cluster create <cluster_name>`.\n")
		return nil
	}

	jetCfg := &jetconfig.Config{
		ConfigVersion: jetconfig.Versions.Prod(),
		Name:          answers.AppName,
		Services:      []jetconfig.Service{},
	}

	if answers.AppType == string(webServiceType) {
		jetCfg.AddNewWebService(answers.AppName + "-" + jetconfig.WebType)
	} else if answers.AppType == string(cronjobServiceType) {
		jetCfg.AddNewCronService(
			answers.AppName+"-"+jetconfig.CronType,
			[]string{"/bin/sh", "-c", "date; echo Hello from Launchpad"},
			"* * * * *",
		)
	}

	if answers.ClusterOption != "" && answers.ClusterOption != createJetpackCluster {
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

func runConfigSurvey(
	ctx context.Context,
	authProvider provider.Auth,
	appName string,
) (*SurveyAnswers, error) {

	clusters, err := cmdOpts.ClusterProvider().GetAll(ctx)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	clusterNames := []string{}
	for _, c := range clusters {
		clusterNames = append(clusterNames, c.GetName())
	}
	// In case user wants to log in and use a jetpack managed cluster.
	clusterNames = append(clusterNames, jetpackManagedCluster)
	// In case user wants to create a cluster.
	clusterNames = append(clusterNames, createJetpackCluster)

	qs, err := surveyQuestions(ctx, appName, clusterNames)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	answers := &SurveyAnswers{}

	err = survey.Ask([]*survey.Question{qs["AppName"]}, &answers.AppName)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	err = survey.Ask([]*survey.Question{qs["AppType"]}, &answers.AppType)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	err = survey.Ask([]*survey.Question{qs["ClusterOption"]}, &answers.ClusterOption)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if answers.ClusterOption == jetpackManagedCluster {
		// Prompt users to log in.
		ctx, err := authProvider.Identify(ctx)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		// Get all the cluster names again.
		clusters, err := cmdOpts.ClusterProvider().GetAll(ctx)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		// Only show the jetpack managed cluster names.
		clusterNames := []string{}
		for _, c := range clusters {
			if c.IsJetpackManaged() {
				clusterNames = append(clusterNames, c.GetName())
			}
		}
		if len(clusterNames) == 0 {
			answers.ClusterOption = createJetpackCluster
		} else {
			// Add option to select creating a new managed cluster.
			clusterNames = append(clusterNames, createJetpackCluster)
			additionalClusterSurvey := surveyJetpackManagedClusterQuestions(ctx, clusterNames)
			err = survey.Ask([]*survey.Question{additionalClusterSurvey["ClusterOption"]}, &answers.ClusterOption)
			if err != nil {
				return nil, errors.WithStack(err)
			}
		}
	}

	return answers, nil
}

func surveyQuestions(ctx context.Context, appName string, clusterNames []string) (map[string]*survey.Question, error) {

	appTypes := jetconfig.GetServiceTypes()
	appTypeOptions := getAppTypeOptions(appTypes)

	questions := map[string]*survey.Question{
		"AppName": {
			Name: "AppName",
			Prompt: &survey.Input{
				Message: "What is the name of this project?",
				Default: appName,
			},
			Validate: func(val any) error {
				if err := survey.MinLength(3)(val); err != nil {
					return err
				}

				nameEntered := val.(string)
				if ok := kubevalidate.IsValidName(nameEntered); !ok {
					// NOTE: we can create an API `kubevalidate.IsValidNameWithReasons` that returns
					// the error messages from k8s.io/apimachinery/pkg/util/validation
					// However, those messages speak about DNS RFC 1035, which may be confusing
					// to our users.

					// The default error text by the Survey lib is prefixed by:
					// X Sorry, your reply was invalid:
					// to which we are adding a suffix of the nameEntered via the %s below.
					// This gives feedback to the user that we actually processed their entered name.
					msg := "%s\n" +
						"For compatibility with kubernetes, we require app names to be:\n" +
						" - less than 64 characters\n" +
						" - consist of lower case alphanumeric characters or '-'\n" +
						" - start with an alphabetic character \n" +
						" - end with an alphanumeric character"
					return fmt.Errorf(msg, nameEntered)
				}
				return nil
			},
		},
		"AppType": {
			Name: "AppType",
			Prompt: &survey.Select{
				Message: "What type of service you would like to add to this project?",
				Options: appTypeOptions,
			},
		},
		"ClusterOption": {
			Name: "ClusterOption",
			Prompt: &survey.Select{
				Message: "To which cluster do you want to deploy this project?",
				Options: clusterNames,
			},
		},
		"ImageRepositoryLocation": {
			Name: "ImageRepositoryLocation",
			Prompt: &survey.Input{
				Message: imageRepositoryFlagHelpMsg,
			},
			Validate: survey.Required,
		},
	}

	return questions, nil
}

func surveyJetpackManagedClusterQuestions(ctx context.Context, clusterNames []string) map[string]*survey.Question {
	return map[string]*survey.Question{
		"ClusterOption": {
			Name: "ClusterOption",
			Prompt: &survey.Select{
				Message: "To which jetpack managed cluster do you want to deploy this project?",
				Options: clusterNames,
			},
		},
	}
}

func appName(path string) (string, error) {
	return kubevalidate.ToValidName(filepath.Base(jetconfig.ConfigDir(path)))
}

func getAppTypeOptions(appTypes []string) []string {
	var appTypeOptions []string
	for _, appType := range appTypes {
		switch appType {
		case jetconfig.WebType:
			appTypeOptions = append(appTypeOptions, string(webServiceType))
		case jetconfig.CronType:
			appTypeOptions = append(appTypeOptions, string(cronjobServiceType))
		}
	}
	return appTypeOptions
}
