package launchpad

import (
	"context"
	"regexp"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/pkg/errors"
	"go.jetpack.io/launchpad/pkg/jetcloud/jetaws"
)

func getAuthenticatedEcrRegistryWithDefaultConfig(
	ctx context.Context,
	imageRegistryUri string,
) (*ImageRegistry, error) {

	awsCfg, err := config.LoadDefaultConfig(ctx)

	if err != nil {
		return nil, errors.WithStack(err)
	}
	dockerCredentials, err := jetaws.DockerCredentials(
		ctx,
		awsCfg,
	)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	registry := &ImageRegistry{
		awsCfg:            &awsCfg,
		dockerCredentials: dockerCredentials,
		host:              awsRegistryHost,
		uri:               imageRegistryUri,
	}
	return registry, nil
}

var accountRegex = regexp.MustCompile(`[0-9]+`)

func createEcrRepository(ctx context.Context, p *PublishPlan) error {
	if p.registry.awsCfg == nil {
		return errAwsConfigIsNilForUserSpecifiedRegistry
	}

	if err := jetaws.EnsureECRRepositoryExists(ctx, *p.registry.awsCfg, p.imageRepository()); err != nil {
		return errors.WithStack(err)
	}

	return jetaws.EnsureECRLifecyclePolicy(
		ctx,
		*p.registry.awsCfg,
		p.imageRepository(),
		accountRegex.FindString(p.registry.uri),
	)
}
