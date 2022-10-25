package hook

import (
	"context"
	"time"

	"go.jetpack.io/launchpad/padcli/helm"
	"go.jetpack.io/launchpad/padcli/provider"
)

type LifecycleOutput interface {
	SetDuration(time.Duration)
}

type commandStartHook func(projectDir string) error
type LifecycleHook func(ctx context.Context, f func() (LifecycleOutput, error))

// Values are the "values" computed to insert into the helm chart
type Values map[string]any

// preHelmValuesComputeHook is called __before__ the helm values computation finishes
type preHelmValuesComputeHook func(
	ctx context.Context,
	providers provider.Providers,
	hvc *helm.ValueComputer,
) error

// postHelmValuesComputeHook is called __after__ the helm values computation finishes
type postHelmValuesComputeHook func(
	ctx context.Context,
	p provider.Providers,
	hvc *helm.ValueComputer,
) (Values, error)

type Hooks struct {
	commandStartHook commandStartHook

	// Deploy Hooks
	preHelmValuesComputeHook   preHelmValuesComputeHook
	postAppChartValuesHook     postHelmValuesComputeHook
	postRuntimeChartValuesHook postHelmValuesComputeHook

	// Lifecycle Hooks
	buildHook   LifecycleHook
	publishHook LifecycleHook
	deployHook  LifecycleHook
}

type hookOption func(*Hooks)

func New(opts ...hookOption) *Hooks {
	h := &Hooks{
		// Initialize lifecycle hooks
		buildHook:   track,
		publishHook: track,
		deployHook:  track,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

func WithCommandStartHook(h commandStartHook) hookOption {
	return func(hooks *Hooks) {
		hooks.commandStartHook = h
	}
}

func WithPreHelmValuesComputeHook(h preHelmValuesComputeHook) hookOption {
	return func(hooks *Hooks) {
		hooks.preHelmValuesComputeHook = h
	}
}

func WithPostAppChartValuesHook(h postHelmValuesComputeHook) hookOption {
	return func(hooks *Hooks) {
		hooks.postAppChartValuesHook = h
	}
}

func WithPostRuntimeChartValuesHook(h postHelmValuesComputeHook) hookOption {
	return func(hooks *Hooks) {
		hooks.postRuntimeChartValuesHook = h
	}
}

func WithBuildHook(h LifecycleHook) hookOption {
	return func(hooks *Hooks) {
		hooks.buildHook = h
	}
}

func WithPublishHook(h LifecycleHook) hookOption {
	return func(hooks *Hooks) {
		hooks.publishHook = h
	}
}

func WithDeployHook(h LifecycleHook) hookOption {
	return func(hooks *Hooks) {
		hooks.deployHook = h
	}
}

func (h *Hooks) CommandStart(
	projectDir string,
) error {
	if h == nil || h.commandStartHook == nil {
		return nil
	}
	return h.commandStartHook(projectDir)
}

func (h *Hooks) PreHelmValuesCompute(
	ctx context.Context,
	p provider.Providers,
	hvc *helm.ValueComputer,
) error {
	if h == nil || h.preHelmValuesComputeHook == nil {
		return nil
	}
	return h.preHelmValuesComputeHook(ctx, p, hvc)
}

func (h *Hooks) PostAppChartValuesCompute(
	ctx context.Context,
	p provider.Providers,
	hvc *helm.ValueComputer,
) (Values, error) {
	if h == nil || h.postAppChartValuesHook == nil {
		return hvc.AppValues(), nil
	}
	return h.postAppChartValuesHook(ctx, p, hvc)
}

func (h *Hooks) PostRuntimeChartValuesCompute(
	ctx context.Context,
	p provider.Providers,
	hvc *helm.ValueComputer,
) (Values, error) {
	if h == nil || h.postRuntimeChartValuesHook == nil {
		return hvc.RuntimeValues(), nil
	}
	return h.postRuntimeChartValuesHook(ctx, p, hvc)
}

func (h *Hooks) Build(ctx context.Context, f func() (LifecycleOutput, error)) {
	h.buildHook(ctx, f)
}

func (h *Hooks) Publish(ctx context.Context, f func() (LifecycleOutput, error)) {
	h.publishHook(ctx, f)
}

func (h *Hooks) Deploy(ctx context.Context, f func() (LifecycleOutput, error)) {
	h.deployHook(ctx, f)
}

func track(ctx context.Context, f func() (LifecycleOutput, error)) {
	st := time.Now()
	out, _ := f()
	duration := time.Since(st)
	out.SetDuration(duration)
}
