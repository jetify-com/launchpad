package launchpad

import (
	"context"
	"fmt"
	"os"

	"go.jetpack.io/launchpad/goutil/errorutil"
	"go.jetpack.io/launchpad/pkg/jetlog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/pkg/errors"
)

type DownOptions struct {
	ExternalCharts []*ChartConfig
	ReleaseName    string
	InstanceName   string
	Namespace      string
	KubeContext    string
}

type helmRelease struct {
	ReleaseName  string
	InstanceName string
	Namespace    string
}

type downPlan struct {
	downOptions *DownOptions
	helmDriver  string
	namespace   string
	releases    []helmRelease
}

func down(ctx context.Context, do *DownOptions) error {

	plan, err := makeDownPlan(ctx, do)
	if err != nil {
		return errors.Wrap(err, "failed to make down plan")
	}

	err = validateDownPlan(plan)
	if err != nil {
		return errors.Wrap(err, "failed to validate down plan")
	}

	err = executeDownPlan(ctx, plan)
	if err != nil {
		return errors.Wrap(err, "failed to execute down plan")
	}
	return nil
}

func makeDownPlan(ctx context.Context, opts *DownOptions) (*downPlan, error) {
	plan := &downPlan{
		helmDriver: os.Getenv("HELM_DRIVER"), // empty is fine
		namespace:  opts.Namespace,
		releases: []helmRelease{
			{
				ReleaseName:  opts.ReleaseName,
				InstanceName: opts.InstanceName,
				Namespace:    opts.Namespace,
			},
		},
		downOptions: opts,
	}

	releases, err := listReleases(
		ctx,
		plan.helmDriver,
		opts.KubeContext,
		opts.Namespace,
	)
	if err != nil {
		return nil, errorutil.CombinedError(err, errUnableToAccessHelmReleases)
	}

	appsInstalled := 0
	appFound := false
	runtimeFound := false
	for _, r := range releases {
		if r.Name == RuntimeChartName {
			runtimeFound = true
		} else {
			appsInstalled++
			if r.Name == opts.ReleaseName {
				appFound = true
			}
		}
	}

	if !appFound {
		return nil,
			errorutil.NewUserErrorf(
				"Could not find %s in namespace %s", opts.InstanceName, opts.Namespace)
	}

	if runtimeFound && appsInstalled > 1 {
		yellow.Fprint(
			jetlog.Logger(ctx),
			"\tFound multiple apps installed. Not removing runtime.",
		)
	}

	if runtimeFound && appsInstalled == 1 {
		plan.releases = append(plan.releases, helmRelease{
			ReleaseName:  RuntimeChartName,
			InstanceName: RuntimeChartName,
			Namespace:    opts.Namespace,
		})
	}

	return plan, nil
}

func validateDownPlan(plan *downPlan) error {
	return nil
}

func executeDownPlan(ctx context.Context, plan *downPlan) error {
	err := uninstallChart(ctx, plan)
	if err != nil {
		return errors.Wrap(err, "failed to helm uninstall chart")
	}
	err = deleteChildResources(ctx, plan)
	if err != nil {
		return errors.Wrap(err, "failed to delete other resources created while app was running")
	}

	return nil
}

func deleteChildResources(ctx context.Context, plan *downPlan) error {
	rc, err := RESTConfigFromDefaults(plan.downOptions.KubeContext)
	if err != nil {
		return errors.Wrap(err, "failed to get k8s client rest config")
	}
	clientset, err := kubernetes.NewForConfig(rc)
	if err != nil {
		return errors.Wrap(err, "failed to create k8s clientset")
	}

	// For now, delete using the typed API for the handful of resource types that
	// we know we create. As that set expands, rewrite to use untyped API.
	selector := metav1.ListOptions{
		LabelSelector: fmt.Sprintf(
			"app.kubernetes.io/instance=%s",
			plan.downOptions.InstanceName,
		),
	}
	namespace := plan.downOptions.Namespace
	err = clientset.CoreV1().Pods(namespace).DeleteCollection(
		ctx,
		metav1.DeleteOptions{},
		selector,
	)
	if err != nil {
		return errors.Wrap(err, "failed to delete pods")
	}
	err = clientset.BatchV1().Jobs(namespace).DeleteCollection(
		ctx,
		metav1.DeleteOptions{},
		selector,
	)
	if err != nil {
		return errors.Wrap(err, "failed to delete jobs")
	}
	err = clientset.BatchV1().CronJobs(namespace).DeleteCollection(
		ctx,
		metav1.DeleteOptions{},
		selector,
	)
	if err != nil {
		return errors.Wrapf(err, "failed to delete cronjobs for ns %s", namespace)
	}
	return nil
}
