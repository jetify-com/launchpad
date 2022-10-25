package launchpad

import (
	"context"
	"strings"

	artifactregistry "cloud.google.com/go/artifactregistry/apiv1beta2"
	"github.com/pkg/errors"
	"go.jetpack.io/launchpad/pkg/jetlog"
	pbartifactregistry "google.golang.org/genproto/googleapis/devtools/artifactregistry/v1beta2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// inspired by code at:
// https://cloud.google.com/go/docs/reference/cloud.google.com/go/artifactregistry/latest/apiv1beta2
func createGcpRepository(ctx context.Context, plan *PublishPlan) error {
	if plan.registry.host != gcpRegistryHost {
		return nil
	}

	c, err := artifactregistry.NewClient(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to make new ArtifactRegistry client")
	}
	defer c.Close()

	repoRequestParent, repoId, err := repoRequestParams(plan)
	if err != nil {
		return errors.Wrap(err, "failed to get repoRequestParams")
	}

	req := &pbartifactregistry.CreateRepositoryRequest{
		Parent: repoRequestParent,
		Repository: &pbartifactregistry.Repository{
			Format: pbartifactregistry.Repository_DOCKER,
		},
		RepositoryId: repoId,
	}

	op, err := c.CreateRepository(ctx, req)
	if err != nil {
		if got, want := status.Code(err), codes.AlreadyExists; got != want {
			return errors.Wrap(err, "failed to create Repo")
		}
		jetlog.Logger(ctx).IndentedPrintf(
			"Repository (%s) exists already in registry (%s). No need to create.\n",
			repoId,
			plan.registry.uri,
		)
		return nil
	}
	jetlog.Logger(ctx).Println("Got Operation for repository creation. Waiting on it to complete now...")

	_ /*resp*/, err = op.Wait(ctx)
	if err != nil {
		return errors.Wrap(err, "failed waiting for the Repo creation operation to finish")
	}
	jetlog.Logger(ctx).IndentedPrintf("Created repository (%s) for registry at %s\n", repoId, plan.registry.uri)
	return nil
}

func repoRequestParams(plan *PublishPlan) (string, string, error) {
	// e.g. plan.registry.uri is us-central1-docker.pkg.dev
	// e.g. region is now us-central1
	region := strings.TrimSuffix(plan.registry.uri, "-docker.pkg.dev")
	// e.g. plan.imageRepo is jetpack-dev/jetpack-internal-4/store/py-dockerfile
	// e.g. project is now jetpack-dev
	imageRepoParts := strings.Split(plan.imageRepo, "/")
	project := imageRepoParts[0]

	parent := "projects/" + project + "/locations/" + region

	// e.g. imageRepoParts is [jetpack-dev, jetpack-internal-4, store, py-dockerfile]
	// e.g. repository is jetpack-internal-4
	repositoryId := imageRepoParts[1] // strings.Join(imageRepoParts[1:], "/")

	return parent, repositoryId, nil
}
