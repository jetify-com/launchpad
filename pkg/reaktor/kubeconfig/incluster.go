/*
Copyright 2021 Jetify Inc.
Copyright 2014 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.

You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

This file has been modified by Jetify Inc.:
+ This an InClusterClientConfig that can be used publicly outside of the k8s
  code. As opposed to the original version which is private.

The code is extracted from:
https://github.com/kubernetes/client-go/blob/master/tools/clientcmd/client_config.go#L539
*/

package kubeconfig

import (
	"fmt"
	"os"
	"strings"

	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// inClusterClientConfig makes a config that will work from within a kubernetes cluster container environment.
// Can take options overrides for flags explicitly provided to the command inside the cluster container.
type inClusterClientConfig struct {
	overrides               *clientcmd.ConfigOverrides
	inClusterConfigProvider func() (*restclient.Config, error)
}

var _ clientcmd.ClientConfig = &inClusterClientConfig{}

func NewInClusterClientConfig(overrides *clientcmd.ConfigOverrides) clientcmd.ClientConfig {
	return &inClusterClientConfig{overrides: overrides}
}

func (config *inClusterClientConfig) RawConfig() (clientcmdapi.Config, error) {
	return clientcmdapi.Config{}, fmt.Errorf("inCluster environment config doesn't support multiple clusters")
}

func (config *inClusterClientConfig) ClientConfig() (*restclient.Config, error) {
	inClusterConfigProvider := config.inClusterConfigProvider
	if inClusterConfigProvider == nil {
		inClusterConfigProvider = restclient.InClusterConfig
	}

	icc, err := inClusterConfigProvider()
	if err != nil {
		return nil, err
	}

	// in-cluster configs only takes a host, token, or CA file
	// if any of them were individually provided, overwrite anything else
	if config.overrides != nil {
		if server := config.overrides.ClusterInfo.Server; len(server) > 0 {
			icc.Host = server
		}
		if len(config.overrides.AuthInfo.Token) > 0 || len(config.overrides.AuthInfo.TokenFile) > 0 {
			icc.BearerToken = config.overrides.AuthInfo.Token
			icc.BearerTokenFile = config.overrides.AuthInfo.TokenFile
		}
		if certificateAuthorityFile := config.overrides.ClusterInfo.CertificateAuthority; len(certificateAuthorityFile) > 0 {
			icc.TLSClientConfig.CAFile = certificateAuthorityFile
		}
	}

	return icc, nil
}

func (config *inClusterClientConfig) Namespace() (string, bool, error) {
	// This way assumes you've set the POD_NAMESPACE environment variable using the downward API.
	// This check has to be done first for backwards compatibility with the way InClusterConfig was originally set up
	if ns := os.Getenv("POD_NAMESPACE"); ns != "" {
		return ns, false, nil
	}

	// Fall back to the namespace associated with the service account token, if available
	if data, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		if ns := strings.TrimSpace(string(data)); len(ns) > 0 {
			return ns, false, nil
		}
	}

	return "default", false, nil
}

func (config *inClusterClientConfig) ConfigAccess() clientcmd.ConfigAccess {
	return clientcmd.NewDefaultClientConfigLoadingRules()
}

// InClusterPossible determines whether it's possible to configure an in-cluster
// client using the service account information.
func InClusterPossible() bool {
	fi, err := os.Stat("/var/run/secrets/kubernetes.io/serviceaccount/token")
	return os.Getenv("KUBERNETES_SERVICE_HOST") != "" &&
		os.Getenv("KUBERNETES_SERVICE_PORT") != "" &&
		err == nil && !fi.IsDir()
}
