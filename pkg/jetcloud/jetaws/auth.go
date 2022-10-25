package jetaws

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/docker/docker/api/types"
	"github.com/pkg/errors"
)

func DockerCredentials(
	ctx context.Context,
	awsCfg aws.Config,
) (string, error) {
	ecrAuthToken, err := getEcrAuthToken(ctx, awsCfg)
	if err != nil {
		return "", errors.WithStack(err)
	}

	const awsUsername = "AWS"
	const expectedTokenPrefix = awsUsername + ":"
	if !strings.HasPrefix(ecrAuthToken, expectedTokenPrefix) {
		return "", errors.New("invalid prefix for token")
	}

	authConfig := types.AuthConfig{
		Username: awsUsername,
		// remove the AWS: prefix from the ecrAuthToken
		Password: ecrAuthToken[len(expectedTokenPrefix):],
	}
	encodedJSON, err := json.Marshal(authConfig)
	if err != nil {
		return "", errors.WithStack(err)
	}
	return base64.URLEncoding.EncodeToString(encodedJSON), nil
}
