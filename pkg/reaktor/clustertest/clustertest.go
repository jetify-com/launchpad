// package clustertest uses `kind` (https://kind.sigs.k8s.io/) to create an in-docker
// testing cluster. Because creating a new cluster can be a little bit slow,
// we lazily create a new cluster that is re-used across all users of the library
// (assuming they are in the same machine).
//
// Since the cluster is shared, make sure your test scope their work on the cluster
// to their particular namespace
//
// If a special environment variable is set it uses
// that cluster instead of creating one. Useful in environments when you want
// to run tests against an existing cluster instead of using `kind`.
//
// Note that this library is meant as a light mechanism to do kubernetes tests
// in our unit tests. If we ever need an end-to-end solution, we should look at
// kubetest2 (https://github.com/kubernetes-sigs/kubetest2)
package clustertest

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/samber/lo"
	"go.jetpack.io/launchpad/pkg/reaktor"
	"go.jetpack.io/launchpad/pkg/reaktor/komponents"
	"go.jetpack.io/launchpad/pkg/reaktor/kubeconfig"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/kind/pkg/cluster"
)

// Environment variable used to specify a pre-existing cluster.
const CLUSTERTEST_KUBECONFIG_PATH = "CLUSTERTEST_KUBECONFIG_PATH"

// Tests that don't need a special image, should just use this default image to
// avoid any issues like rate throttling.
func DefaultImage() string {
	// For now we set to amazonlinux since our tests run on AWS. A future version
	// of this function could return a different image based on what cloud we're
	// running in.
	return "public.ecr.aws/amazonlinux/amazonlinux:latest"
}

type _testCluster struct {
	sync.Mutex
	created    bool
	kubeConfig string
}

// Singleton: we create the test cluster once and share the instance.
var testCluster = &_testCluster{
	created: false,
}

// Keep this name in sync with devtools/scripts/write-kubeconfigs.sh
const testClusterName = "clustertest"

// Convenience function that gets a client builder for the test cluster, or
// panics.
//
// This is useful in unittest where this call is expected to succeed or
// the test should fail anyways.
func ClientBuilder() kubeconfig.ClientBuilder {
	envvar := os.Getenv(CLUSTERTEST_KUBECONFIG_PATH)
	if envvar != "" {
		return kubeconfig.FromFile(envvar)
	}

	cluster, err := getTestCluster()
	if err != nil {
		panic(err)
	}

	// TODO: provide an easier way to create a builder when using a loader and flags.
	builder := kubeconfig.NewClientBuilder(
		kubeconfig.WithLoader(kubeconfig.YamlLoader{
			YAML: cluster.kubeConfig,
		}),
		kubeconfig.WithFlags(&kubeconfig.Flags{
			Insecure: kubeconfig.Bool(true),
		}),
	)
	return builder
}

func MustInitTestCluster() {
	if os.Getenv(CLUSTERTEST_KUBECONFIG_PATH) != "" {
		return // Nothing to initialize if using a pre-specified cluster
	}

	// Ensure the kind cluster exists:
	_, err := getTestCluster()
	if err != nil {
		panic(err)
	}
}

func getTestCluster() (*_testCluster, error) { // Keep small since it locks
	// If tests are running concurrently, lock to avoid trying to create the
	// cluster multiple times.
	// TODO: Should this be a cross-process, filesystem-based lock
	testCluster.Lock()
	defer testCluster.Unlock()

	if testCluster.created {
		return testCluster, nil
	}

	provider := cluster.NewProvider()
	err := ensureTestClusterExists(provider, testClusterName)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	kubeConfig, err := provider.KubeConfig(testClusterName, false /* internal */)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	// Because we are running docker-in-docker we need to connect to the host machine,
	// instead of to localhost (aka current container).
	// https://host.docker.internal works out of the box for OS X (so it should just
	// work on our toast environments).
	//
	// In linux we need to set it up with a command-line flag when starting the docker
	// container (for example, in CI/CD). --add-host=host.docker.internal:host-gateway
	// See: https://github.com/moby/moby/pull/40007
	//
	// A slight complication is that the created cluster does not have https://host.docker.internal
	// as a valid hostname in its internal certificates, so we need to do insecure
	// connections with it (with kubectl we need to use --insecure-skip-tls-verify)
	// Since this are ephemeral test clusters, we think that's ok for now.
	//
	// Lastly the user running the tests needs to have permissions to use
	// docker-in-docker (by having permissions on /var/run/docker.sock)
	testCluster.kubeConfig = strings.ReplaceAll(kubeConfig, "server: https://127.0.0.1", "server: https://host.docker.internal")
	testCluster.created = true // Wait until the very end before setting to true
	return testCluster, nil
}

func ensureTestClusterExists(provider *cluster.Provider, clusterName string) error {
	exists, err := doesTestClusterExist(provider, clusterName)
	if err != nil {
		return errors.WithStack(err)
	}

	if !exists {
		err = provider.Create(
			testClusterName,
			cluster.CreateWithWaitForReady(10*time.Second), // Timeout after that
		)
		if err != nil {
			return errors.WithStack(err)
		}
	}
	return nil
}

func doesTestClusterExist(provider *cluster.Provider, clusterName string) (bool, error) {
	clusters, err := provider.List()
	if err != nil {
		return false, errors.WithStack(err)
	}
	return lo.Contains(clusters, clusterName), nil
}

func Namespace(
	ctx context.Context,
	klient *reaktor.Reaktor,
) (*komponents.Namespace, func() error, error) {
	// To avoid tests interfering with each other, each test needs to operate in
	// its own namespace and clean up afterwards. We use a randomly generated
	// namespace for that.
	ns := &komponents.Namespace{
		Name: fmt.Sprintf("%s--%s", "clustertest", utilrand.String(44)),
	}
	// and ensure it will be cleaned up
	cleanupFunc := func() error {
		err := klient.Delete(ctx, ns)
		if err != nil {
			return errors.WithStack(err)
		}
		return nil
	}

	// Check namespace doesn't exist
	_, err := klient.GetByResource(ctx, ns)
	if !k8serrors.IsNotFound(err) {
		return ns, cleanupFunc, errors.WithStack(err)
	}

	// Create namespace:
	_, err = klient.Apply(ctx, ns)
	if err != nil {
		return ns, cleanupFunc, errors.WithStack(err)
	}

	return ns, cleanupFunc, nil
}
