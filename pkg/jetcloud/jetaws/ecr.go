package jetaws

import (
	"context"
	_ "embed"
	"encoding/base64"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/pkg/errors"
	"github.com/samber/lo"
)

//go:embed ecr-lifecycle-policy.json
var ecrLifecyclePolicy string

func EnsureECRRepositoryExists(
	ctx context.Context,
	awsCfg aws.Config,
	repoPath string,
	tags ...types.Tag,
) error {
	ecrClient := ecr.NewFromConfig(awsCfg)
	_, err := ecrClient.CreateRepository(ctx, &ecr.CreateRepositoryInput{
		RepositoryName: &repoPath,
		Tags:           tags,
	})
	var existsErr *types.RepositoryAlreadyExistsException
	if errors.As(err, &existsErr) {
		return nil
	}
	return errors.Wrapf(err, "error creating ECR repo: %v", repoPath)
}

func EnsureECRRepoHasTags(
	ctx context.Context,
	awsCfg aws.Config,
	repoPath string,
	account string,
	tags ...types.Tag,
) {
	ecrClient := ecr.NewFromConfig(awsCfg)
	arn := fmt.Sprintf("arn:aws:ecr:%s:%s:repository/%s", awsCfg.Region, account, repoPath)
	_, err := ecrClient.TagResource(ctx, &ecr.TagResourceInput{
		// arn is of the form arn:aws:ecr:region:012345678910:repository/path/to/repo
		ResourceArn: lo.ToPtr(arn),
		Tags:        tags,
	})

	if err != nil {
		fmt.Printf("Error tagging ECR repository with arn: %s : %s\n", arn, err)
	}
}

func EnsureECRLifecyclePolicy(
	ctx context.Context,
	awsCfg aws.Config,
	repoPath string,
	account string,
) error {
	ecrClient := ecr.NewFromConfig(awsCfg)
	_, err := ecrClient.PutLifecyclePolicy(ctx, &ecr.PutLifecyclePolicyInput{
		LifecyclePolicyText: lo.ToPtr(ecrLifecyclePolicy),
		RepositoryName:      lo.ToPtr(repoPath),
		RegistryId:          lo.ToPtr(account),
	})
	return errors.WithStack(err)
}

func getEcrAuthToken(
	ctx context.Context,
	awsCfg aws.Config,
) (string, error) {

	ecrSvc := ecr.NewFromConfig(awsCfg)
	output, err := ecrSvc.GetAuthorizationToken(ctx, &ecr.GetAuthorizationTokenInput{})
	if err != nil {
		return "", errors.WithStack(err)
	}

	authToken := *output.AuthorizationData[0].AuthorizationToken
	decodedAuthToken, err := base64.StdEncoding.DecodeString(authToken)
	if err != nil {
		return "", errors.Wrap(err, "failed to decode auth token from AWS")
	}

	return string(decodedAuthToken), nil
}
