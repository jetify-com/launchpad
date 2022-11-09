package provider

import (
	"context"
)

type Repository interface {
	// string here is the image repository address (excluding the image tag).
	// It is left as "" if using the default provided image repository
	Get(context.Context, Cluster, string) (RepoConfig, error)
}

type RepoConfig interface {
	GetCredentials() string
	GetImageRepoPrefix() string
	GetCloudCredentials() any
	GetRegion() string
}

type emptyRepository struct {
}

func EmptyRepository() Repository {
	return &emptyRepository{}
}

func (p *emptyRepository) Get(
	ctx context.Context,
	c Cluster,
	imageRepo string,
) (RepoConfig, error) {
	return nil, nil
}
