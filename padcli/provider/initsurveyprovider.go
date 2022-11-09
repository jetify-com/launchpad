package provider

import (
	"context"
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/pkg/errors"
	"go.jetpack.io/launchpad/padcli/jetconfig"
	"go.jetpack.io/launchpad/pkg/kubevalidate"
)

type serviceTypeOption string

const (
	WebServiceType             serviceTypeOption = "Web Service"
	CronjobServiceType         serviceTypeOption = "Cron Job"
	JetpackManagedCluster      string            = "Jetpack managed cluster"
	CreateJetpackCluster       string            = "Create a new cluster with Jetpack"
	ImageRepositoryFlagHelpMsg                   = "Image repository to push the built image to. " +
		"Your kubernetes cluster must have the permissions to pull images from this repository."
)

type SurveyAnswers struct {
	AppName                 string
	AppType                 string
	ClusterOption           string
	KubeContext             string
	ImageRepositoryLocation string
}

type InitSurveyProvider interface {
	Run(
		ctx context.Context,
		authProvider Auth,
		appName string,
	) (*SurveyAnswers, error)
}

type initSurveyProvider struct {
	ClusterProvider ClusterProvider
}

func DefaultInitSurveyProvider(cp ClusterProvider) InitSurveyProvider {
	return &initSurveyProvider{cp}
}

func (p *initSurveyProvider) Run(
	ctx context.Context,
	authProvider Auth,
	appName string,
) (*SurveyAnswers, error) {

	clusters, err := p.ClusterProvider.GetAll(ctx)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	clusterNames := []string{}
	for _, c := range clusters {
		clusterNames = append(clusterNames, c.GetName())
	}
	// In case user wants to log in and use a jetpack managed cluster.
	clusterNames = append(clusterNames, JetpackManagedCluster)
	// In case user wants to create a cluster.
	clusterNames = append(clusterNames, CreateJetpackCluster)

	qs, err := surveyQuestions(ctx, appName, clusterNames)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	answers := &SurveyAnswers{}

	// These questions could be moved to the survey provider
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

	return answers, nil
}

func surveyQuestions(
	ctx context.Context,
	appName string,
	clusterNames []string,
) (map[string]*survey.Question, error) {

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
				Message: ImageRepositoryFlagHelpMsg,
			},
			Validate: survey.Required,
		},
	}

	return questions, nil
}

func getAppTypeOptions(appTypes []string) []string {
	var appTypeOptions []string
	for _, appType := range appTypes {
		switch appType {
		case jetconfig.WebType:
			appTypeOptions = append(appTypeOptions, string(WebServiceType))
		case jetconfig.CronType:
			appTypeOptions = append(appTypeOptions, string(CronjobServiceType))
		}
	}
	return appTypeOptions
}
