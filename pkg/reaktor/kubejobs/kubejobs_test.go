package kubejobs

import (
	"context"
	"testing"

	utilrand "k8s.io/apimachinery/pkg/util/rand"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.jetpack.io/launchpad/pkg/reaktor"
	"go.jetpack.io/launchpad/pkg/reaktor/clustertest"
	"go.jetpack.io/launchpad/pkg/reaktor/komponents"
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

func (suite *Suite) TestWaitUntilCompletion() {
	assert := assert.New(suite.T())

	ctx := context.Background()

	// Initialize a client on our testing cluster
	builder := clustertest.ClientBuilder()
	klient, err := reaktor.WithClientBuilder(builder)
	if !assert.NoError(err) {
		return
	}

	ns, nsCleanupFn, err := clustertest.Namespace(ctx, klient)
	if !assert.NoError(err) {
		return
	}
	defer func() {
		err = nsCleanupFn()
		assert.NoError(err)
	}()

	// Create a resource
	job := &komponents.Job{
		Namespace:      ns.Name,
		Name:           "test-job-" + utilrand.String(5),
		ContainerImage: clustertest.DefaultImage(),
		Command:        []string{"echo", "This is a job"},
	}

	unstructuredJob, err := klient.Apply(ctx, job)
	if !assert.NoError(err) {
		return
	}

	w, err := klient.WatchUntil(ctx, unstructuredJob, func(e watch.Event) (bool, error) {
		result, err := IsJobTerminated(e, unstructuredJob)
		return result, errors.WithStack(err)
	})
	if assert.NoError(err) {
		assert.NotNil(w)
		assert.Equal(w.Type, watch.Modified)
	}
}

func (suite *Suite) TestWaitUntilCompletionWithTwoJobs() {
	assert := assert.New(suite.T())

	ctx := context.Background()

	// Initialize a client on our testing cluster
	builder := clustertest.ClientBuilder()
	klient, err := reaktor.WithClientBuilder(builder)
	if !assert.NoError(err) {
		return
	}

	ns, nsCleanupFn, err := clustertest.Namespace(ctx, klient)
	if !assert.NoError(err) {
		return
	}
	defer func() {
		err = nsCleanupFn()
		assert.NoError(err)
	}()

	// create a resource that runs for a few seconds
	jobSleeping := &komponents.Job{
		Namespace:      ns.Name,
		Name:           "test-job-" + utilrand.String(5),
		ContainerImage: clustertest.DefaultImage(),
		Command:        []string{"sleep", "2"},
	}
	unstructuredJobSleeping, err := klient.Apply(ctx, jobSleeping)
	if !assert.NoError(err) {
		return
	}

	// Create a resource that runs and completes "quickly"
	jobQuick := &komponents.Job{
		Namespace:      ns.Name,
		Name:           "test-job-" + utilrand.String(5),
		ContainerImage: clustertest.DefaultImage(),
		Command:        []string{"echo", "This is a job"},
	}

	_ /*unstructuredJobQuick*/, err = klient.Apply(ctx, jobQuick)
	if !assert.NoError(err) {
		return
	}

	w, err := klient.WatchUntil(ctx, unstructuredJobSleeping, func(e watch.Event) (bool, error) {
		result, err := IsJobTerminated(e, unstructuredJobSleeping)
		return result, errors.WithStack(err)
	})
	if assert.NoError(err) {
		assert.NotNil(w)
		assert.Equal(w.Type, watch.Modified)
	}
}

// TODO Add more test-cases. Can use some from:
// https://github.com/kubernetes/kubernetes/blob/master/test/e2e/framework/job/fixtures.go
func (suite *Suite) TestWaitUntilErrorEvent() {

	testCases := map[string]struct {
		command    []string
		errMessage string
	}{
		"backoffLimitExceeded": {
			command:    []string{"/bin/sh", "-c", "exit 1"},
			errMessage: BackoffLimitExceededErr,
		},
		"deadlineExceeded": {
			command:    []string{"sleep", "20"},
			errMessage: DeadlineExceededErr,
		},
	}

	for name, testCase := range testCases {
		suite.T().Run(name, func(t *testing.T) {
			testWaitUntilErrorEvent(t, testCase.command, testCase.errMessage)
		})
	}
}

func testWaitUntilErrorEvent(t *testing.T, command []string, errMessage string) {

	assert := assert.New(t)

	ctx := context.Background()

	// Initialize a client on our testing cluster
	builder := clustertest.ClientBuilder()
	klient, err := reaktor.WithClientBuilder(builder)
	assert.NoError(err)

	ns, nsCleanupFn, err := clustertest.Namespace(ctx, klient)
	assert.NoError(err)
	defer func() {
		err = nsCleanupFn()
		assert.NoError(err)
	}()

	// Create a resource
	job := &komponents.Job{
		Namespace:      ns.Name,
		Name:           "test-job-" + utilrand.String(5),
		ContainerImage: clustertest.DefaultImage(),

		Command: command,
	}
	job.PatchForTest(map[string]any{
		"spec": map[string]any{
			"activeDeadlineSeconds": 10,
		},
	})

	unstructuredJob, err := klient.Apply(ctx, job)
	if !assert.NoError(err) {
		return
	}

	watchEvt, err := klient.WatchUntil(ctx, unstructuredJob, func(e watch.Event) (bool, error) {
		result, err := IsJobTerminated(e, unstructuredJob)
		return result, errors.WithStack(err)
	})
	assert.Nil(err)
	isFailedWithReason, err := IsJobFailedWithReason(watchEvt)
	assert.True(isFailedWithReason.IsFailed)
	assert.Contains(isFailedWithReason.Reason, errMessage)
}
