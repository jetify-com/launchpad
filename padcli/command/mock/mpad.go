package mock

import (
	"context"

	"github.com/stretchr/testify/mock"
	"go.jetpack.io/launchpad/launchpad"
)

type MockPad struct {
	mock.Mock
	BuildFunc       func(ctx context.Context, opts *launchpad.BuildOptions) (*launchpad.BuildOutput, error)
	DeployFunc      func(ctx context.Context, opts *launchpad.DeployOptions) (*launchpad.DeployOutput, error)
	PublishFunc     func(ctx context.Context, opts *launchpad.PublishOptions) (*launchpad.PublishOutput, error)
	DownFunc        func(ctx context.Context, do *launchpad.DownOptions) error
	PortForwardFunc func(ctx context.Context, opts *launchpad.PortForwardOptions) error
}

func (p *MockPad) Build(ctx context.Context, opts *launchpad.BuildOptions) (*launchpad.BuildOutput, error) {
	if p.BuildFunc != nil {
		return p.BuildFunc(ctx, opts)
	}
	return &launchpad.BuildOutput{}, nil
}

func (p *MockPad) Deploy(ctx context.Context, opts *launchpad.DeployOptions) (*launchpad.DeployOutput, error) {
	if p.DeployFunc != nil {
		return p.DeployFunc(ctx, opts)
	}
	return &launchpad.DeployOutput{}, nil
}

func (p *MockPad) Publish(
	ctx context.Context,
	opts *launchpad.PublishOptions,
) (*launchpad.PublishOutput, error) {
	if p.PublishFunc != nil {
		return p.PublishFunc(ctx, opts)
	}
	return &launchpad.PublishOutput{}, nil
}

func (p *MockPad) Down(ctx context.Context, do *launchpad.DownOptions) error {
	if p.DownFunc != nil {
		return p.DownFunc(ctx, do)
	}
	return nil
}

func (p *MockPad) PortForward(ctx context.Context, opts *launchpad.PortForwardOptions) error {
	if p.PortForwardFunc != nil {
		return p.PortForwardFunc(ctx, opts)
	}
	return nil
}
