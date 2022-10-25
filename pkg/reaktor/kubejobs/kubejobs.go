// kubejobs adds APIs specific to k8s jobs
package kubejobs

import (
	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
)

// These can be moved outside this package into reaktor once we use
// them for resources other than Jobs. I want one other use-case before doing
// that refactoring.
const (
	BackoffLimitExceededErr = "BackoffLimitExceeded"
	DeadlineExceededErr     = "DeadlineExceeded"
)

func IsJobTerminated(e watch.Event, targetJob *unstructured.Unstructured) (bool, error) {
	if e.Type != watch.Modified {
		if e.Type == watch.Added {
			return false, nil
		}

		return true, errors.Errorf("did not expect job watch.Event to be %s", e.Type)
	}

	job, err := jobFromEvent(&e)
	if err != nil {
		return false, errors.Wrapf(
			err,
			"failed isJobFinished for job: %s.%s",
			job.GetName(),
			job.GetNamespace(),
		)
	}
	if targetJob.GetName() != job.GetName() {
		return false, errors.Errorf(
			"targetJob (%s) and eventJob's (%s) names don't match",
			targetJob.GetName(),
			job.GetName(),
		)
	}
	for _, c := range job.Status.Conditions {
		if c.Status == "True" &&
			(c.Type == batchv1.JobComplete || c.Type == batchv1.JobFailed) {
			return true, nil
		}
	}
	return false, nil
}

func IsJobCompleted(e *watch.Event) (bool, error) {
	isFailedWithReason, err := IsJobFailedWithReason(e)
	if err != nil {
		return false, errors.WithStack(err)
	}
	return !isFailedWithReason.IsFailed, nil
}

// IsFailedWithReason returns a boolean regarding whether the job failed to
// complete. It provides a best-effort Reason for the failure, if kubernetes
// api provides one.
type IsFailedWithReason struct {
	IsFailed bool
	Reason   string
}

func IsJobFailedWithReason(e *watch.Event) (*IsFailedWithReason, error) {
	job, err := jobFromEvent(e)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	for _, c := range job.Status.Conditions {
		if c.Status != "True" {
			continue
		}

		if c.Type == batchv1.JobComplete {
			return &IsFailedWithReason{false, ""}, nil
		} else if c.Type == batchv1.JobFailed {
			return &IsFailedWithReason{true, c.Reason}, nil
		}
	}
	return &IsFailedWithReason{false, ""}, nil
}

func jobFromEvent(e *watch.Event) (*batchv1.Job, error) {
	obj := e.Object
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return nil, errors.Errorf(
			"Unable to cast the runtime.Object to an unstructured.Unstructured. "+
				"The object's type is: %T",
			obj,
		)
	}

	// Note that helm has a similar function that uses a `convertWithMapper`.
	// It is possible that handles some scenarios which our implementation
	// (using the k8s lib) doesn't handle.
	// If there are any bugs, or discrepancies, this may be worth looking into.
	var job *batchv1.Job
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &job)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return job, nil
}
