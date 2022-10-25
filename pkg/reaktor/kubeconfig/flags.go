/*
Copyright 2021 Jetpack Technologies Inc.
Copyright 2014 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.

You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

This file has been modified by Jetpack Technologies Inc to isolate flags that
can be used when configuring a kubernetes client. The code is extracted from:
https://github.com/kubernetes/cli-runtime/blob/master/pkg/genericclioptions/config_flags.go#L80
*/

package kubeconfig

import (
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Flags struct {
	CacheDir    string   // Default cache directory
	ReuseConfig BoolFlag // If true, caches the config file on load and re-uses it. Instead of loading repeatedly.

	// Flags – all the flags below map to a corresponding `kubectl` flag.
	ClusterName      string   // The name of the kubeconfig cluster to use
	AuthInfoName     string   // The name of the kubeconfig user to use
	Context          string   // The name of the kubeconfig context to use
	Namespace        string   // If present, the namespace scope for requests from this client.
	APIServer        string   // The address and port of the Kubernetes API server
	TLSServerName    string   // Server name to use for server certificate validation. If it is not provided, the hostname used to contact the server is used
	Insecure         BoolFlag // If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure"
	CertFile         string   // Path to a client certificate file for TLS
	KeyFile          string   // Path to a client key file for TLS
	CAFile           string   // Path to a cert file for the certificate authority
	BearerToken      string   // Bearer token for authentication to the API server
	Impersonate      string   // Username to impersonate for the operation
	ImpersonateGroup []string // Groups to impersonate for the operation.
	Username         string   // Username for basic authentication to the API server
	Password         string   // Password for basic authentication to the API server
	Timeout          string   // The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests.

	// If non-nil, wrap config function can transform the Config
	// before it is returned in ToRESTConfig function.
	WrapConfigFn func(*rest.Config) *rest.Config
}

// Convenience type so that we can distinguish when the flag has been set vs not
// set.
type BoolFlag *bool

func Bool(value bool) BoolFlag {
	return &value
}

func DefaultFlags() *Flags {
	impersonateGroup := []string{}

	return &Flags{
		ReuseConfig: Bool(true),
		Insecure:    Bool(false),
		Timeout:     "0",

		CacheDir:         defaultCacheDir,
		ImpersonateGroup: impersonateGroup,
	}
}

func (f *Flags) ToOverrides() *clientcmd.ConfigOverrides {
	overrides := &clientcmd.ConfigOverrides{ClusterDefaults: clientcmd.ClusterDefaults}

	// Auth info flags:
	if f.CertFile != "" {
		overrides.AuthInfo.ClientCertificate = f.CertFile
	}
	if f.KeyFile != "" {
		overrides.AuthInfo.ClientKey = f.KeyFile
	}
	if f.BearerToken != "" {
		overrides.AuthInfo.Token = f.BearerToken
	}
	if f.Impersonate != "" {
		overrides.AuthInfo.Impersonate = f.Impersonate
	}
	if len(f.ImpersonateGroup) > 0 {
		overrides.AuthInfo.ImpersonateGroups = f.ImpersonateGroup
	}
	if f.Username != "" {
		overrides.AuthInfo.Username = f.Username
	}
	if f.Password != "" {
		overrides.AuthInfo.Password = f.Password
	}

	// Cluster flags:
	if f.APIServer != "" {
		overrides.ClusterInfo.Server = f.APIServer
	}
	if f.TLSServerName != "" {
		overrides.ClusterInfo.TLSServerName = f.TLSServerName
	}
	if f.CAFile != "" {
		overrides.ClusterInfo.CertificateAuthority = f.CAFile
	}
	if f.Insecure != nil {
		overrides.ClusterInfo.InsecureSkipTLSVerify = *f.Insecure
	}

	// Context flags:
	if f.Context != "" {
		overrides.CurrentContext = f.Context
	}
	if f.ClusterName != "" {
		overrides.Context.Cluster = f.ClusterName
	}
	if f.AuthInfoName != "" {
		overrides.Context.AuthInfo = f.AuthInfoName
	}
	if f.Namespace != "" {
		overrides.Context.Namespace = f.Namespace
	}

	if f.Timeout != "" {
		overrides.Timeout = f.Timeout
	}

	return overrides
}

func (f *Flags) override(newFlags *Flags) {
	if newFlags.CacheDir != "" {
		f.CacheDir = newFlags.CacheDir
	}

	if newFlags.ReuseConfig != nil {
		f.ReuseConfig = newFlags.ReuseConfig
	}

	// Auth info flags:
	if newFlags.CertFile != "" {
		f.CertFile = newFlags.CertFile
	}
	if newFlags.KeyFile != "" {
		f.KeyFile = newFlags.KeyFile
	}
	if newFlags.BearerToken != "" {
		f.BearerToken = newFlags.BearerToken
	}
	if newFlags.Impersonate != "" {
		f.Impersonate = newFlags.Impersonate
	}
	if len(f.ImpersonateGroup) > 0 {
		f.ImpersonateGroup = newFlags.ImpersonateGroup
	}
	if newFlags.Username != "" {
		f.Username = newFlags.Username
	}
	if newFlags.Password != "" {
		f.Password = newFlags.Password
	}

	// Cluster flags:
	if newFlags.APIServer != "" {
		f.APIServer = newFlags.APIServer
	}
	if newFlags.TLSServerName != "" {
		f.TLSServerName = newFlags.TLSServerName
	}
	if newFlags.CAFile != "" {
		f.CAFile = newFlags.CAFile
	}
	if newFlags.Insecure != nil {
		f.Insecure = newFlags.Insecure
	}

	// Context flags:
	if newFlags.Context != "" {
		f.Context = newFlags.Context
	}
	if newFlags.ClusterName != "" {
		f.ClusterName = newFlags.ClusterName
	}
	if newFlags.AuthInfoName != "" {
		f.AuthInfoName = newFlags.AuthInfoName
	}
	if newFlags.Namespace != "" {
		f.Namespace = newFlags.Namespace
	}

	if newFlags.Timeout != "" {
		f.Timeout = newFlags.Timeout
	}
}
