package reaktor_test

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	utilrand "k8s.io/apimachinery/pkg/util/rand"

	"github.com/stretchr/testify/suite"
	"go.jetpack.io/launchpad/pkg/reaktor"
	"go.jetpack.io/launchpad/pkg/reaktor/clustertest"
	"go.jetpack.io/launchpad/pkg/reaktor/komponents"
	"go.jetpack.io/launchpad/pkg/reaktor/kubejobs"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
)

type Suite struct {
	suite.Suite
}

func (suite *Suite) SetupSuite() {
	clustertest.MustInitTestCluster()
}

func TestSuite(t *testing.T) {
	t.Skip("Skipping until we fix cluster DEV-729")
	suite.Run(t, &Suite{})
}

func (suite *Suite) TestApply() {
	// TODO: Generalize some of the logic here so it can be easily used by other
	// tests.
	ctx := context.Background()
	req := suite.Require()

	// Initialize a client on our testing cluster
	builder := clustertest.ClientBuilder()
	klient, err := reaktor.WithClientBuilder(builder)
	req.NoError(err)

	ns, nsCleanupFn, err := clustertest.Namespace(ctx, klient)
	req.NoError(err)
	defer func() {
		err = nsCleanupFn()
		req.NoError(err)
	}()

	// Create a resource
	job := &komponents.Job{
		Name:           "test-job-" + utilrand.String(5),
		Namespace:      ns.Name,
		ContainerImage: clustertest.DefaultImage(),
		Command:        []string{"echo", "This is a job!"},
	}

	result, err := klient.Apply(ctx, job)
	req.NoError(err)

	manifest, err := reaktor.ToManifest(job)
	req.NoError(err)

	// Check that the job specified and the job created correspond to each other.
	// For now we just check a few fields. A better approach might to check that
	// each field in the manifest exists and has the same value in the result.
	// We can't check that manifest and result are equal because result comes
	// back with a lot of additional metadata that isn't part of the original spec.
	req.Equal(manifest.GetKind(), result.GetKind())
	req.Equal(manifest.GetName(), result.GetName())
	req.Equal(manifest.GetNamespace(), result.GetNamespace())
}

// TestFindPodSpecAndCreateJob will:
// 1. Start a job (this is setup; alternatively could start a service, cronjob, etc.)
// 2. Fetch the PodSpec from the job's pod.
// 3. Start a new job with the PodSpec.
// 4. Verify this new job completes.
func (suite *Suite) TestFindPodSpecAndCreateJob() {
	req := suite.Require()
	ctx := context.Background()

	//
	// setup
	//

	// Initialize a client on our testing cluster
	builder := clustertest.ClientBuilder()
	klient, err := reaktor.WithClientBuilder(builder)
	req.NoError(err)

	ns, nsCleanupFn, err := clustertest.Namespace(ctx, klient)
	req.NoError(err)
	defer func() {
		err = nsCleanupFn()
		req.NoError(err)
	}()

	suite.createJobAndWaitTillTermination(ctx, ns, klient)

	//
	// end setup
	//

	// At this point, the job has completed running, so we can fetch the podSpec
	podSpec := suite.getPodSpecOfJob(ctx, klient, ns.Name)

	// now, we can start a new job!
	jobWithSpec := &komponents.JobWithSpec{
		Command:   []string{"echo", "ran job-with-spec-in-test"},
		Name:      "job-with-spec-in-test",
		Namespace: ns.Name,
		PodSpec:   podSpec,
	}
	unstrJobWithSpec, err := klient.Apply(ctx, jobWithSpec)
	req.NoError(err)

	req.NotNil(unstrJobWithSpec)
	watchEvt, err := klient.WatchUntil(
		ctx, unstrJobWithSpec, func(e watch.Event) (bool, error) {
			result, err := kubejobs.IsJobTerminated(e, unstrJobWithSpec)
			return result, errors.WithStack(err)
		},
	)
	req.NoError(err)
	req.NotNil(watchEvt)
	req.Equal(watchEvt.Type, watch.Modified)

	isCompleted, err := kubejobs.IsJobCompleted(watchEvt)
	req.NoError(err)
	req.True(isCompleted)
}

func (suite *Suite) createJobAndWaitTillTermination(
	ctx context.Context,
	ns *komponents.Namespace,
	klient *reaktor.Reaktor,
) *unstructured.Unstructured {
	req := suite.Require()

	// Create a resource
	job := &komponents.Job{
		Name:           "test-job-" + utilrand.String(5),
		Namespace:      ns.Name,
		ContainerImage: clustertest.DefaultImage(),
		Command:        []string{"echo", "This is a job!"},
	}

	unstructuredJob, err := klient.Apply(ctx, job)
	req.NoError(err)

	manifest, err := reaktor.ToManifest(job)
	req.NoError(err)

	// Check that the job specified and the job created correspond to each other.
	// For now we just check a few fields. A better approach might to check that
	// each field in the manifest exists and has the same value in the unstructuredJob.
	// We can't check that manifest and unstructuredJob are equal because unstructuredJob comes
	// back with a lot of additional metadata that isn't part of the original spec.
	req.Equal(manifest.GetKind(), unstructuredJob.GetKind())
	req.Equal(manifest.GetName(), unstructuredJob.GetName())
	req.Equal(manifest.GetNamespace(), unstructuredJob.GetNamespace())

	w, err := klient.WatchUntil(
		ctx, unstructuredJob, func(e watch.Event) (bool, error) {
			result, err := kubejobs.IsJobTerminated(e, unstructuredJob)
			return result, errors.WithStack(err)
		},
	)
	req.NoError(err)
	req.NotNil(w)
	req.Equal(w.Type, watch.Modified)
	return manifest
}

func (suite *Suite) getPodSpecOfJob(
	ctx context.Context,
	klient *reaktor.Reaktor,
	ns string,
) map[string]any {
	req := suite.Require()

	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	pods, err := klient.List(ctx, gvr, ns, v1.ListOptions{})
	req.NoError(err)
	req.True(
		len(pods.Items) == 1,
		"expect one pod in the namespace the job ran in but got %d",
		len(pods.Items),
	)
	jobPod := pods.Items[0]

	// Get the PodSpec from the jobPod
	untypedPodSpec, found, err := unstructured.NestedFieldCopy(jobPod.Object, "spec")
	req.True(found, "expect to find the spec in the jobPod")
	req.NoError(err)
	req.NotNil(untypedPodSpec)
	return untypedPodSpec.(map[string]any)
}
