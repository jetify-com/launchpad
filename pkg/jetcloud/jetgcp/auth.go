package jetgcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"golang.org/x/oauth2"
	oauth2google "golang.org/x/oauth2/google"
)

// keep this in sync with pulumi script at provisioning/services/compiler.ts
// TODO migrate over to using all env variables?
const smarthopServiceAccountJsonKeyFilePath = "/home/smarthop_artifact_registry/creds.json"

const smarthopServiceAccountEnv = "SMARTHOP_ARTIFACT_REGISTRY_CREDENTIALS"

func getOauthToken(ctx context.Context, scopes []string) (*oauth2.Token, error) {
	base64JSON := viper.GetString(smarthopServiceAccountEnv)
	json, err := base64.StdEncoding.DecodeString(base64JSON)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if len(json) == 0 {
		json, err = os.ReadFile(smarthopServiceAccountJsonKeyFilePath)
		if err != nil {
			return nil, errors.WithStack(err)
		}
	}
	creds, err := oauth2google.CredentialsFromJSON(ctx, json, scopes...)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	token, err := creds.TokenSource.Token()
	return token, errors.WithStack(err)
}

func DockerCredentials(ctx context.Context) (string, error) {

	scopes := []string{
		"https://www.googleapis.com/auth/cloud-platform",
	}
	token, err := getOauthToken(ctx, scopes)
	if err != nil {
		return "", errors.WithStack(err)
	}

	authConfig := types.AuthConfig{
		Username: "oauth2accesstoken",
		Password: token.AccessToken,
	}
	jsonAuthConfig, err := json.Marshal(authConfig)
	if err != nil {
		return "", errors.WithStack(err)
	}
	return base64.URLEncoding.EncodeToString(jsonAuthConfig), nil
}
