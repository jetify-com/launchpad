package provider

import (
	"context"
)

type Repository interface {
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
