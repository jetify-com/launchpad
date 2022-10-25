// Copyright 2022 Jetpack Technologies Inc and contributors. All rights reserved.
// Use of this source code is governed by the license in the LICENSE file.

package padcli

import (
	"context"

	"github.com/spf13/cobra"
	"go.jetpack.io/launchpad/padcli/command"
	"go.jetpack.io/launchpad/padcli/flags"
	"go.jetpack.io/launchpad/padcli/hook"
	"go.jetpack.io/launchpad/padcli/provider"
)

type Padcli struct {
	additionalCommands []*cobra.Command
	analyticsProvider  provider.Analytics
	authProvider       provider.Auth
	clusterProvider    provider.ClusterProvider
	envSecProvider     provider.EnvSec
	errorLogger        provider.ErrorLogger
	hooks              *hook.Hooks
	persistentPreRunE  func(cmd *cobra.Command, args []string) error
	persistentPostRunE func(cmd *cobra.Command, args []string) error
	namespaceProvider  provider.NamespaceProvider
	repositoryProvider provider.Repository
	rootCommand        *cobra.Command
	rootFlags          *flags.RootCmdFlags
}
type padcliOption func(*Padcli)

func New(opts ...padcliOption) *Padcli {
	p := &Padcli{
		hooks:              hook.New(),
		rootFlags:          &flags.RootCmdFlags{},
		analyticsProvider:  provider.DefaultAnalyticsProvider(),
		authProvider:       provider.Anonymous(),
		clusterProvider:    provider.KubeConfigClusterProvider(),
		envSecProvider:     provider.DefaultEnvSecProvider(),
		namespaceProvider:  provider.KubeConfigNamespaceProvider(),
		repositoryProvider: provider.EmptyRepository(),
		errorLogger:        &provider.NoOpLogger{},
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *Padcli) Run(ctx context.Context) {
	command.Execute(ctx, p)
}

func (p *Padcli) AnalyticsProvider() provider.Analytics {
	return p.analyticsProvider
}

func (p *Padcli) AuthProvider() provider.Auth {
	return p.authProvider
}

func (p *Padcli) ClusterProvider() provider.ClusterProvider {
	return p.clusterProvider
}

func (p *Padcli) EnvSecProvider() provider.EnvSec {
	return p.envSecProvider
}

func (p *Padcli) ErrorLogger() provider.ErrorLogger {
	return p.errorLogger
}

func (p *Padcli) Hooks() *hook.Hooks {
	return p.hooks
}

func (p *Padcli) NamespaceProvider() provider.NamespaceProvider {
	return p.namespaceProvider
}

func (p *Padcli) RepositoryProvider() provider.Repository {
	return p.repositoryProvider
}

func (p *Padcli) RootFlags() *flags.RootCmdFlags {
	return p.rootFlags
}

func (p *Padcli) RootCommand() *cobra.Command {
	if p.rootCommand == nil {
		p.rootCommand = command.NewRootCmd(p)
	}
	return p.rootCommand
}

func (p *Padcli) AdditionalCommands() []*cobra.Command {
	return p.additionalCommands
}

func (p *Padcli) PersistentPreRunE(cmd *cobra.Command, args []string) error {
	if p == nil || p.persistentPreRunE == nil {
		return nil
	}
	return p.persistentPreRunE(cmd, args)
}

func (p *Padcli) PersistentPostRunE(cmd *cobra.Command, args []string) error {
	if p == nil || p.persistentPostRunE == nil {
		return nil
	}
	return p.persistentPostRunE(cmd, args)
}

// Options
type cmdFunc func(pad *Padcli) *cobra.Command

func WithAdditionalCommands(cmds ...cmdFunc) padcliOption {
	return func(p *Padcli) {
		for _, cmd := range cmds {
			p.additionalCommands = append(p.additionalCommands, cmd(p))
		}
	}
}

func WithAnalyticsProvider(analytics provider.Analytics) padcliOption {
	return func(p *Padcli) {
		p.analyticsProvider = analytics
	}
}

func WithAuthProvider(auth provider.Auth) padcliOption {
	return func(p *Padcli) {
		p.authProvider = auth
	}
}

func WithClusterProvider(provider provider.ClusterProvider) padcliOption {
	return func(p *Padcli) {
		p.clusterProvider = provider
	}
}

func WithEnvSecProvider(provider provider.EnvSec) padcliOption {
	return func(p *Padcli) {
		p.envSecProvider = provider
	}
}

func WithErrorLogger(logger provider.ErrorLogger) padcliOption {
	return func(p *Padcli) {
		p.errorLogger = logger
	}
}

func WithHooks(hooks *hook.Hooks) padcliOption {
	return func(p *Padcli) {
		p.hooks = hooks
	}
}

func WithNamespaceProvider(ns provider.NamespaceProvider) padcliOption {
	return func(p *Padcli) {
		p.namespaceProvider = ns
	}
}

func WithPersistentPreRunE(r func(cmd *cobra.Command, args []string) error) padcliOption {
	return func(p *Padcli) {
		p.persistentPreRunE = r
	}
}

func WithPersistentPostRunE(r func(cmd *cobra.Command, args []string) error) padcliOption {
	return func(p *Padcli) {
		p.persistentPostRunE = r
	}
}

func WithRepositoryProvider(provider provider.Repository) padcliOption {
	return func(p *Padcli) {
		p.repositoryProvider = provider
	}
}
