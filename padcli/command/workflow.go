package command

// This is a stopgap file until we build a Workflow API that
// can compose a sequence of launchpad steps with pretty user-facing messages

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"go.jetpack.io/launchpad/launchpad"
	"go.jetpack.io/launchpad/padcli/jetconfig"
	"go.jetpack.io/launchpad/padcli/provider"
	"go.jetpack.io/launchpad/pkg/jetlog"
)

func execLaunchpadBuild(
	ctx context.Context,
	pad launchpad.LaunchPad,
	buildOpts *embeddedBuildOptions,
	jetCfg *jetconfig.Config,
	cluster provider.Cluster,
	absProjPath string,
	repoConfig provider.RepoConfig,
	stepInfo string, // make an abstraction for this.
) (*launchpad.BuildOptions, *launchpad.BuildOutput, error) {
	jetlog.Logger(ctx).HeaderPrintf("Building project %s", jetCfg.GetProjectName())
	jetlog.Logger(ctx).HeaderPrintf("Step %s Building Docker image...", stepInfo)

	opts, err := makeBuildOptions(ctx, buildOpts, jetCfg, cluster, absProjPath, repoConfig)
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}
	bo, err := pad.Build(ctx, opts)
	if err != nil {
		return nil, nil, errors.Wrap(err, "Error building")
	}
	if bo.DidBuildUsingDockerfile() {
		jetlog.Logger(ctx).HeaderPrintf(
			"[DONE] Successfully built Docker image in %s\n",
			bo.Duration.Truncate(time.Millisecond*100),
		)
	} else {
		jetlog.Logger(ctx).HeaderPrintf(
			"[DONE] Success. No Docker image needed to be built.\n")
	}

	return opts, bo, nil
}

func execLaunchpadPublish(
	ctx context.Context,
	pad launchpad.LaunchPad,
	buildOutput *launchpad.BuildOutput,
	imageRepoOverride string,
	repoConfig provider.RepoConfig,
	cluster provider.Cluster,
	jetCfg *jetconfig.Config,
	stepInfo string,
) (*launchpad.PublishOptions, *launchpad.PublishOutput, error) {

	jetlog.Logger(ctx).HeaderPrintf("Step %s: Publishing Images", stepInfo)

	var publishOutput *launchpad.PublishOutput

	// Only publish for non-local clusters or if a repo is explicitly specified
	if !cluster.IsLocal() || imageRepoOverride != "" {
		pubOpts, err := makePublishOptions(imageRepoOverride, repoConfig, buildOutput, jetCfg)
		if err != nil {
			return nil, nil, errors.WithStack(err)
		}
		if len(pubOpts.LocalImages) > 0 {
			publishOutput, err = pad.Publish(ctx, pubOpts)
			if err != nil {
				return nil, nil, errors.Wrap(err, "failed to publish")
			}
		}
	}

	if publishOutput.DidPublish() {
		jetlog.Logger(ctx).HeaderPrintf(
			"[DONE] Docker images published in %s\n",
			publishOutput.Duration.Truncate(time.Millisecond*100),
		)
	} else {
		jetlog.Logger(ctx).HeaderPrintf("[DONE] No need to publish Docker images.\n")
	}

	return nil, publishOutput, nil
}
