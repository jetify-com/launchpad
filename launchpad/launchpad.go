package launchpad

import (
	"context"

	"github.com/pkg/errors"
	"github.com/spf13/afero"
	"github.com/stern/stern/stern"
	"go.jetpack.io/launchpad/padcli/hook"
	"go.jetpack.io/launchpad/padcli/provider"
)

// Mike question: Why do we have individual interfaces?

// This file has the public interface to the launchpad package
type Builder interface {
	Build(ctx context.Context, opts *BuildOptions) (*BuildOutput, error)
}

type Publisher interface {
	Publish(ctx context.Context, opts *PublishOptions) (*PublishOutput, error)
}

type Deployer interface {
	Deploy(ctx context.Context, opts *DeployOptions) (*DeployOutput, error)
}

type LaunchPad interface {
	Builder
	Publisher
	Deployer
	Down(ctx context.Context, do *DownOptions) error
	PortForward(ctx context.Context, opts *PortForwardOptions) error
}

// Pad has the foundational elements on top of which the LaunchPad is constructed.
type Pad struct {
	// fs is the filesystem
	fs          afero.Fs
	errorLogger provider.ErrorLogger
}

func NewPad(errorLogger provider.ErrorLogger) *Pad {
	return &Pad{
		fs:          afero.NewOsFs(),
		errorLogger: errorLogger,
	}
}

func NewPadForTest() *Pad {
	return &Pad{
		fs: afero.NewMemMapFs(),
	}
}

// Build uses the Dockerfile to build a docker image of the module
func (p *Pad) Build(
	ctx context.Context,
	opts *BuildOptions,
) (*BuildOutput, error) {
	var err error
	var out *BuildOutput
	opts.LifecycleHook(ctx, func() (hook.LifecycleOutput, error) {
		out, err = build(ctx, p.fs, opts)
		return out, err
	})
	return out, err
}

func (p *Pad) RunLocally(
	ctx context.Context,
	opts *LocalOptions,
) error {
	return p.CreateAndStartContainerInLocalMode(ctx, opts)
}

// ToManifest produces Helm Charts
func (p *Pad) ToManifest() error {
	return nil
}

// If an image-registry is specified, Publish will
// send the docker image built by Build() to a docker-registry
// TODO(Landau) rename to push so more closely resemble `docker push` and
// reduce ambiguity with publishing as a means to make something public.
func (p *Pad) Publish(
	ctx context.Context,
	opts *PublishOptions,
) (*PublishOutput, error) {
	var err error
	var out *PublishOutput
	opts.LifecycleHook(ctx, func() (hook.LifecycleOutput, error) {
		out, err = p.publishDockerImage(ctx, opts)
		return out, err
	})
	return out, err
}

// Deploy creates kubernetes resources from the Manifest
func (p *Pad) Deploy(
	ctx context.Context,
	opts *DeployOptions,
) (*DeployOutput, error) {
	var err error
	var out *DeployOutput
	opts.LifecycleHook(ctx, func() (hook.LifecycleOutput, error) {
		out, err = p.deploy(ctx, opts)
		return out, err
	})
	return out, err
}

func (p *Pad) Down(ctx context.Context, do *DownOptions) error {
	return errors.Wrap(down(ctx, do), "failed launchpad.down")
}

func (p *Pad) PortForward(ctx context.Context, opts *PortForwardOptions) error {
	return errors.WithStack(p.portForward(ctx, opts))
}

func (p *Pad) TailLogs(ctx context.Context, kubeCtx string, do *DeployOutput) error {
	if err := validateRelease(do, AppChartName); err != nil {
		return errors.Wrap(err, "unable to tail logs after deployment")
	}
	return tailLogs(ctx, kubeCtx, do.Namespace, do.InstanceName, do.Releases[AppChartName].Version, stern.RUNNING)
}

func (p *Pad) TailLogsOnErr(ctx context.Context, kubeCtx string, do *DeployOutput) error {
	if err := validateRelease(do, AppChartName); err != nil {
		return errors.Wrap(err, "unable to tail logs for failed deployment")
	}
	return tailLogsForApp(ctx, kubeCtx, do.Namespace, do.InstanceName, do.Releases[AppChartName].Version, stern.TERMINATED)
}

func (p *Pad) TailLogsForAppExec(ctx context.Context, kubeCtx string, do *DeployOutput) error {
	if err := validateRelease(do, AppChartName); err != nil {
		return errors.Wrap(err, "unable to tail logs after deployment")
	}
	return tailLogsForExec(ctx, kubeCtx, do.Namespace, do.InstanceName, do.Releases[AppChartName].Version)
}

func validateRelease(do *DeployOutput, chartName string) error {
	if do == nil || do.Releases == nil || do.Releases[chartName] == nil {
		return errNoDeployRelease
	}
	return nil
}
