/*
Copyright 2021 Jetpack Technologies Inc.
Copyright 2014 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.

You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

This file has been modified by Jetpack Technologies Inc to:
  - Support different ClientConfigs, depending on whether the kubeconfig is
    being provided via a file, in-cluster, or the contents are passed in directly.
  - The RESTClientGetter interface has been generalized to a ClientBuilder interface
    that also has helper methods for creating Dynamic Clients and Clientsets.

The original code is extracted from:
https://github.com/kubernetes/cli-runtime/blob/master/pkg/genericclioptions/config_flags.go
*/
package kubeconfig

import (
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	diskcached "k8s.io/client-go/discovery/cached/disk"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

var (
	defaultCacheDir = filepath.Join(homedir.HomeDir(), ".kube", "cache")
)

// TODO conform to pkg/cmd/util/Factory
// TODO rename to ClientFactory
type ClientBuilder interface {
	ToRESTConfig() (*rest.Config, error)
	ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error)
	ToRESTMapper() (meta.RESTMapper, error)
	ToRawClientConfig() (clientcmd.ClientConfig, error) // Changed from genercclioptions, means interfaces don't match perfectly.
	ToDynamicClient() (dynamic.Interface, error)
	ToClientset() (*kubernetes.Clientset, error)
	ToRawKubeConfigLoader() clientcmd.ClientConfig
}

// clientbuilder implements interface ClientBuilder (compile-time check)
var _ ClientBuilder = (*clientbuilder)(nil)

type clientbuilder struct {
	// Configurable via constructor (using options):
	loader Loader // Interface
	flags  *Flags

	// Internal fields:
	clientConfig clientcmd.ClientConfig
	lock         sync.Mutex

	// Allows increasing burst used for discovery, this is useful
	// in clusters with many registered resources
	discoveryBurst int
}

func NewClientBuilder(opts ...Option) *clientbuilder {
	builder := &clientbuilder{
		loader: DefaultLoader{},
		flags:  DefaultFlags(),
		// The more groups you have, the more discovery requests you need to make.
		// given 25 groups (our groups + a few custom resources) with one-ish version each, discovery needs to make 50 requests
		// double it just so we don't end up here again for a while.  This config is only used for discovery.
		discoveryBurst: 100,
	}

	// Set options
	for _, opt := range opts {
		opt(builder)
	}

	return builder
}

type Option func(*clientbuilder)

func WithLoader(loader Loader) Option {
	return func(b *clientbuilder) { b.loader = loader }
}

func WithFlags(flags *Flags) Option {
	return func(b *clientbuilder) { b.flags.override(flags) }
}

// If WrapConfigFn is non-nil this function can transform config before return.
func (b *clientbuilder) ToRESTConfig() (*rest.Config, error) {
	rc, err := b.toRawClientConfig()
	if err != nil {
		return nil, err
	}
	c, err := rc.ClientConfig()
	if err != nil {
		return nil, err
	}
	if b.flags.WrapConfigFn != nil {
		return b.flags.WrapConfigFn(c), nil
	}
	return c, nil
}

func (b *clientbuilder) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	// We add this so that ClientBuilder can conform to RESTClientGetter.
	// This lets us invoke NewFactory(clientBuilder), and pass this factory
	// into reaktor.
	//
	// In the future, we should make ClientBuilder implement the Factory
	// interface, so that reaktor can directly use a ClientBuilder, and avoid
	// calling this ToRawKubeConfigLoader method.
	//
	// Also - see that ToRawClientConfig has the same implementation of
	// what ToRawKubeConfigLoader should have, with the important difference
	// that it returns an error (rather than handling the error lazily).
	panic("not implementing ToRawKubeConfigLoader")
}

func (b *clientbuilder) ToRawClientConfig() (clientcmd.ClientConfig, error) {
	// In the original code this function was called ToRawKubeConfigLoader()
	// but I find that confusing, especially after we introduced Loader.
	// Renamed ToRawClientConfig instead.
	//
	// Also - this implementation returns an error.

	if b.flags.ReuseConfig != nil && *b.flags.ReuseConfig {
		return b.toCachedRawClientConfig()
	}
	return b.toRawClientConfig()
}

func (b *clientbuilder) toRawClientConfig() (clientcmd.ClientConfig, error) {
	overrides := b.flags.ToOverrides()
	return b.loader.clientConfig(overrides)
}

func (b *clientbuilder) toCachedRawClientConfig() (clientcmd.ClientConfig, error) {
	// In the original code this function was called ToRawPersistentKubeConfigLoader()
	// We think the term 'Cached' is more accurate.
	b.lock.Lock()
	defer b.lock.Unlock()

	if b.clientConfig == nil {
		clientConfig, err := b.toRawClientConfig()
		if err != nil {
			return nil, err
		}
		b.clientConfig = clientConfig
	}

	return b.clientConfig, nil
}

// Returns a CachedDiscoveryInterface using a computed RESTConfig.
func (b *clientbuilder) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	config, err := b.ToRESTConfig()
	if err != nil {
		return nil, err
	}

	// The more groups you have, the more discovery requests you need to make.
	// given 25 groups (our groups + a few custom resources) with one-ish version each, discovery needs to make 50 requests
	// double it just so we don't end up here again for a while.  This config is only used for discovery.
	config.Burst = b.discoveryBurst

	cacheDir := defaultCacheDir

	// retrieve a user-provided value for the "cache-dir"
	// override httpCacheDir and discoveryCacheDir if user-value is given.
	if b.flags.CacheDir != "" {
		cacheDir = b.flags.CacheDir
	}
	httpCacheDir := filepath.Join(cacheDir, "http")
	discoveryCacheDir := computeDiscoverCacheDir(filepath.Join(cacheDir, "discovery"), config.Host)

	return diskcached.NewCachedDiscoveryClientForConfig(config, discoveryCacheDir, httpCacheDir, 10*time.Minute)
}

// ToRESTMapper returns a mapper.
func (b *clientbuilder) ToRESTMapper() (meta.RESTMapper, error) {
	discoveryClient, err := b.ToDiscoveryClient()
	if err != nil {
		return nil, err
	}

	mapper := restmapper.NewDeferredDiscoveryRESTMapper(discoveryClient)
	expander := restmapper.NewShortcutExpander(mapper, discoveryClient)
	return expander, nil
}

func (b *clientbuilder) ToDynamicClient() (dynamic.Interface, error) {
	cfg, err := b.ToRESTConfig()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return dynamic.NewForConfig(cfg)
}

func (b *clientbuilder) ToClientset() (*kubernetes.Clientset, error) {
	cfg, err := b.ToRESTConfig()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return kubernetes.NewForConfig(cfg)
}

// overlyCautiousIllegalFileCharacters matches characters that *might* not be supported.  Windows is really restrictive, so this is really restrictive
var overlyCautiousIllegalFileCharacters = regexp.MustCompile(`[^(\w/\.)]`)

// computeDiscoverCacheDir takes the parentDir and the host and comes up with a "usually non-colliding" name.
func computeDiscoverCacheDir(parentDir, host string) string {
	// strip the optional scheme from host if its there:
	schemelessHost := strings.Replace(strings.Replace(host, "https://", "", 1), "http://", "", 1)
	// now do a simple collapse of non-AZ09 characters.  Collisions are possible but unlikely.  Even if we do collide the problem is short lived
	safeHost := overlyCautiousIllegalFileCharacters.ReplaceAllString(schemelessHost, "_")
	return filepath.Join(parentDir, safeHost)
}
