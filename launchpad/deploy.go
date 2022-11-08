package launchpad

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	gotime "time"

	"github.com/imdario/mergo"
	"github.com/pkg/errors"
	"github.com/sethvargo/go-password/password"
	"go.jetpack.io/launchpad/goutil"
	"go.jetpack.io/launchpad/goutil/errorutil"
	"go.jetpack.io/launchpad/padcli/hook"
	"go.jetpack.io/launchpad/padcli/jetconfig"
	"go.jetpack.io/launchpad/pkg/buildstamp"
	"go.jetpack.io/launchpad/pkg/jetlog"
	"go.jetpack.io/launchpad/proto/api"
	"golang.org/x/sync/errgroup"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/time"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	chartRepoName    = "jetpack-chart-repo"
	AppChartName     = "app"
	RuntimeChartName = "jetpack-runtime"

	ApiKeySecretName = "api-key-secret"
)

var appChartVersion = buildstamp.StableDockerTag
var runtimeChartVersion = buildstamp.StableDockerTag

// TODO: fix spacing in this struct
type DeployOptions struct {
	App *HelmOptions

	Environment string // api.Environment

	ExternalCharts []*ChartConfig

	IsLocalCluster bool

	// We should remove this jetconfig dependency. Pass in an interface. This will be easier to design
	// once we have a few service types implemented, and we understand the concrete
	// requirements. Leaving jetconfig in for now.
	JetCfg *jetconfig.Config

	KubeContext string

	LifecycleHook hook.LifecycleHook

	Namespace string

	RemoteEnvVars map[string]string
	Runtime       *HelmOptions

	SecretFilePaths []string

	ReinstallOnHelmUpgradeError bool
}

type ChartConfig struct {
	Repo      string
	Name      string
	Namespace string
	Release   string // unique identifier for installation (can be same or different from display name)
	Timeout   gotime.Duration
	Wait      bool

	chartLocation string // optional path to local chart
	chartVersion  string
	instanceName  string // resources will inherit this name
	values        map[string]any
}

func (c *ChartConfig) HumanName() string {
	return goutil.Coalesce(c.instanceName, c.Name)
}

type DeployPlan struct {
	DeployOptions      *DeployOptions
	appChartConfig     *ChartConfig
	runtimeChartConfig *ChartConfig
	helmDriver         string
}

func (dp *DeployPlan) Charts() []*ChartConfig {
	// Order matters
	charts := []*ChartConfig{}
	if dp.runtimeChartConfig != nil {
		charts = append(charts, dp.runtimeChartConfig)
	}
	if dp.appChartConfig != nil {
		charts = append(charts, dp.appChartConfig)
	}
	return charts
}

type DeployOutput struct {
	Duration     gotime.Duration
	InstanceName string
	Namespace    string
	Releases     map[string]*release.Release // keyed by unique chart name
}

func (do *DeployOutput) AppPort() int {

	// Get the value set by the user, if any
	if port, ok := do.Releases[AppChartName].Config["podPort"]; ok {
		return port.(int)
	}

	// Read the value from the Chart's default values.yaml file.
	// This is more correct than the const used below.
	//
	// For some reason, golang feels these are floats
	if floatYourPort, ok := do.Releases[AppChartName].Chart.Values["podPort"].(float64); ok {
		// this truncates the float i.e. 5.4 becomes 5,
		// which doesn't matter for Port values which are whole numbers
		return int(floatYourPort)
	}

	// All else failing, fallback to the expected default value
	return defaultPodPort
}

func (do *DeployOutput) SetDuration(d gotime.Duration) {
	if do != nil {
		do.Duration = d
	}
}

func (p *Pad) deploy(
	ctx context.Context,
	opts *DeployOptions,
) (*DeployOutput, error) {
	plan, err := p.makeDeployPlan(ctx, opts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to make deploy plan")
	}

	err = validateDeployPlan(plan)
	if err != nil {
		return nil, errors.Wrap(err, "failed to validate deploy plan")
	}

	releases, err := executeDeployPlan(ctx, plan)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to execute deploy plan")
	}

	return &DeployOutput{
		InstanceName: plan.appChartConfig.instanceName,
		Namespace:    plan.appChartConfig.Namespace,
		Releases:     releases,
	}, nil
}

func (p *Pad) makeDeployPlan(
	ctx context.Context,
	opts *DeployOptions,
) (*DeployPlan, error) {
	envVars := map[string]string{}
	for name, value := range opts.RemoteEnvVars {
		envVars[name] = base64.StdEncoding.EncodeToString([]byte(value))
	}

	// if secrets are already set in helmOptions from env-override flag
	// then merge them with secrets from parameter store with priority on env-override values
	if _, ok := opts.App.Values["secrets"]; ok {
		err := mergo.Merge(&envVars, opts.App.Values["secrets"], mergo.WithOverride)
		if err != nil {
			return nil, errors.Wrap(err, "unable to merge .env file values with jetpack env values")
		}
	}

	secretsToMountAsFiles, err := loadSecretFiles(opts.SecretFilePaths)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load secret data from files: %v", opts.SecretFilePaths)
	}

	ttlSecondsAfterFinished := 86400 // 24 hours
	if strings.EqualFold(opts.Environment, api.Environment_DEV.String()) {
		ttlSecondsAfterFinished = 600 // 10 minutes, if dev
	}

	// Any value that is defaulted in helm/app/values.yaml should probably
	// have strutil.NilIfEmpty() applied here. Otherwise, passing an empty string
	// will remove the default.
	appValues := goutil.FilterStringKeyMap(map[string]any{
		"image": opts.App.Values["image"],
		"jetpack": map[string]any{
			"instanceName": opts.App.InstanceName,
			"environment":  opts.Environment,
			"sdkBinPath":   nil,
		},
		"serviceAccount": map[string]any{
			"annotations": map[string]any{
				"eks.amazonaws.com/role-arn": nil,
			},
		},
		"secrets":               envVars, // store envVars using k8s secrets
		"secretsToMountAsFiles": secretsToMountAsFiles,
		"jobs": map[string]any{
			"ttlSecondsAfterFinished": ttlSecondsAfterFinished,
		},
	})

	if err := mergo.Merge(&appValues, opts.App.Values, mergo.WithAppendSlice); err != nil {
		return nil, errors.Wrap(err, "unable to merge value maps")
	}

	helmDriver := os.Getenv("HELM_DRIVER")
	plan := &DeployPlan{
		DeployOptions: opts,
		helmDriver:    helmDriver, // empty is fine.
	}

	// chart config for user app
	plan.appChartConfig = &ChartConfig{
		chartLocation: opts.App.ChartLocation,
		Name:          AppChartName,
		chartVersion:  appChartVersion,
		instanceName:  opts.App.InstanceName,
		Release:       opts.App.ReleaseName,
		Namespace:     opts.Namespace,
		values:        appValues,
		Wait:          true,
		Timeout:       goutil.Coalesce(opts.App.Timeout, defaultHelmTimeout),
	}

	if opts.Runtime == nil {
		// No need to install runtime chart.
		return plan, nil
	}

	runtimeValues := map[string]any{
		"image": map[string]any{}, // pre-create for convenience
		"redis": map[string]any{
			// "password": this is set later only if needed
			// See https://github.com/bitnami/charts/tree/master/bitnami/redis#cluster-topologies
			// for possible cluster topologies. For now using standalone for simplicity
			// but this can be changed to replication if needed
			"architecture": "standalone",
			"auth": map[string]any{
				// This felt more secure than hardcoding a password here.
				// When I tried this without using a file (using env instead)
				// it was not working. Not sure if it was some sort of race condition
				// with secret creation or something. Once I switched to using password
				// file it worked as expected.
				"existingSecret":            RuntimeChartName,
				"existingSecretPasswordKey": "redis-pass",
				"usePasswordFiles":          true,
			},
			"master": map[string]any{
				"configuration": "notify-keyspace-events K$z",
				"persistence": map[string]any{
					"enabled": opts.IsLocalCluster,
				},
			},
		},
		"jetpack": map[string]any{
			// This is set later, only if it is missing. This prevents us from
			// overriding a pre-existing api-key's secret value.
			// "apiKeySecret": "",
		},
	}
	buildstmp := buildstamp.Get()
	if buildstmp.IsDevBinary() {
		// Dev defaults to latest because the actual version might not exist.
		// In practice, this means in development the runtime might not update automatically.
		// use --helm.runtime.set image.tag=[tag] to override
		// Question: Should this go in cmd package instead?
		runtimeValues["image"].(map[string]any)["tag"] = "latest"
	}

	if err := mergo.Merge(&runtimeValues, opts.Runtime.Values, mergo.WithAppendSlice, mergo.WithOverride); err != nil {
		return nil, errors.Wrap(err, "unable to merge value maps")
	}

	runtimeChartConfig := &ChartConfig{
		chartLocation: opts.Runtime.ChartLocation,
		Name:          RuntimeChartName,
		chartVersion:  runtimeChartVersion,
		Release:       RuntimeChartName, // Not a mistake, name and install name are the same
		instanceName:  RuntimeChartName, // Not a mistake, name and instance name are the same
		Namespace:     opts.Namespace,
		values:        runtimeValues,
		Wait:          true,
		Timeout:       goutil.Coalesce(opts.Runtime.Timeout, defaultHelmTimeout),
	}
	// use the specified kube-context name, if any
	settings := newSettings(plan.DeployOptions.KubeContext)

	runtimeIsCurrent := func() bool {
		isInstalled, err := chartIsInstalledAndCurrent(
			ctx,
			helmDriver,
			runtimeChartConfig,
			settings,
			// This is a bit ugly and fragile. Any values added after checking if
			// runtime is installed need to be filtered out so that
			// chartIsInstalledAndCurrent can compare current values to previous
			// release. Benefit of adding the values after this check are
			// a) performance - no need to query k8s
			// b) security - no need for secrets to be fetched
			[]valueKeyPath{
				[]string{"redis", "password"},
				[]string{"jetpack", "apiKeySecret"},
			},
		)

		return err == nil && isInstalled
	}

	// Optimization: skip jetpack-runtime if we can.
	installRuntime, _ := appValues["jetpack"].(map[string]any)["runSDKRegister"].(bool)
	if installRuntime && runtimeIsCurrent() {
		jetlog.Logger(ctx).IndentedPrintf(
			"\nSkipping upgrade of %s because there are no changes\n",
			runtimeChartConfig.Release,
		)
	} else if installRuntime {
		secretData, err := GetRuntimeSecretData(ctx, opts.KubeContext, opts.Namespace)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		redisPass, err := getOrCreateRedisPass(secretData)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		// WARNING: See comment in runtimeIsCurrent before adding more values here
		runtimeChartConfig.values["redis"].(map[string]any)["password"] = redisPass
		if apiKey, ok := secretData[ApiKeySecretName]; ok {
			runtimeChartConfig.values["jetpack"].(map[string]any)["apiKeySecret"] =
				base64.StdEncoding.EncodeToString(apiKey)
		}
		plan.runtimeChartConfig = runtimeChartConfig
		// END OF WARNING
	}

	return plan, nil
}

func validateDeployPlan(dp *DeployPlan) error {
	for _, chart := range dp.Charts() {
		if err := chart.validate(); err != nil {
			return errors.Wrap(err, "Deploy plan failed to validate")
		}
	}

	// TODO ensure that custom-namespace is not set if we are using the prod-trial cluster
	return nil
}

func (cc *ChartConfig) validate() error {
	if cc.Name == "" {
		return errors.Wrap(errInvalidChartConfig, "Chart Config is missing chart name")
	}
	if cc.Release == "" {
		return errors.Wrap(errInvalidChartConfig, "Chart Config is missing install name")
	}
	if cc.instanceName == "" {
		return errors.Wrap(errInvalidChartConfig, "Chart Config is missing instance name")
	}
	if cc.Namespace == "" {
		return errors.Wrap(errInvalidChartConfig, "Chart Config is missing a namespace")
	}
	return nil
}

func executeDeployPlan(
	ctx context.Context,
	dp *DeployPlan,
) (map[string]*release.Release, error) {
	errGroup, errGroupCTX := errgroup.WithContext(ctx)

	// Watch for errors in the deployed containers to see if we need to cancel the deploy.
	// This can happen (for example) if a python-dependency has not been added to requirements.txt
	errGroup.Go(func() error {
		return errors.WithStack(watchForContainerErrors(errGroupCTX, dp))
	})

	var errFinishedApplyHelm = errors.New("Finished applying helm charts")
	var releases map[string]*release.Release

	// Apply the Helm charts for the DeployPlan
	errGroup.Go(func() error {
		rs, err := applyHelmCharts(errGroupCTX, dp)
		if err != nil {
			return errors.WithStack(err)
		}

		releases = rs
		// We return this as a whitelisted error so that the errGroup will cancel the other
		// goroutine that watches for container errors
		return errFinishedApplyHelm
	})

	err := errGroup.Wait()
	if errors.Is(err, errFinishedApplyHelm) {
		// This is our whitelisted error used for errGroup goroutines to terminate cleanly
		// so we can clear the error at this point.
		err = nil
	}

	return releases, errors.Wrap(err, "failed to apply helm charts")
}

func loadFileDataBase64(path string) (string, error) {
	if path == "" {
		return "", nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", errorutil.CombinedError(err, errInvalidFile)
	}
	base64EncodedData := base64.StdEncoding.EncodeToString(data)

	return base64EncodedData, nil
}

func loadSecretFiles(paths []string) (map[string]string, error) {
	// The secret file map has filename as key and base64 content as value
	secretsToMountAsFiles := map[string]string{}

	for _, path := range paths {
		if path != "" {
			filename := filepath.Base(path)
			if _, ok := secretsToMountAsFiles[filename]; ok {
				return nil, errors.WithStack(errors.Errorf("conflicting secret file names : supplied secret files paths with identical name %s: %s", filename, path))
			}

			encodedData, err := loadFileDataBase64(path)
			if err != nil {
				return secretsToMountAsFiles, errors.Wrapf(err, "failed to load secret data from file %s", path)
			}
			secretsToMountAsFiles[filename] = encodedData
		}
	}

	return secretsToMountAsFiles, nil
}

type valueKeyPath []string

// chartIsInstalledAndCurrent returns true if all these conditions are met:
//  1. An existing and active release of this chart already exists.
//  2. The chart version used by the release is the same as that in cc.
//  3. The user-provided values used by the release are the same as those in cc.
//  4. There were no errors in fetching helm data to validate the prior conditions.
//  5. The runtime was last deployed over 24-hours ago. This is a short-cut we're taking
//     to allow us to pin the runtime version to a fixed number and not risk having users
//     run stale runtimes for too long. Later, we should actually bump runtime versions
//     on both the server and the CLI side, and remove this check.
func chartIsInstalledAndCurrent(
	ctx context.Context,
	helmDriver string,
	cc *ChartConfig,
	settings *cli.EnvSettings,
	ignoredCurrentValues []valueKeyPath,
) (bool, error) {
	currentRelease, err := getRelease(ctx, helmDriver, cc, settings)
	if err != nil {
		return false, errors.WithStack(err)
	}
	if currentRelease.Info.LastDeployed.Add(24*gotime.Hour).Before(time.Now()) ||
		currentRelease.Info.Status != release.StatusDeployed ||
		currentRelease.Chart.Metadata.Version != runtimeChartVersion {
		return false, nil
	}
	currentValues, err := getValues(ctx, helmDriver, cc, settings)
	if err != nil {
		return false, errors.WithStack(err)
	}

	for _, path := range ignoredCurrentValues {
		goutil.DigDelete(currentValues, path...)
	}

	return reflect.DeepEqual(cc.values, currentValues), nil
}

func getOrCreateRedisPass(secretData map[string][]byte) (string, error) {
	pass, ok := secretData["redis-pass"]
	// If redis pass is not here, it means secret does not exist so redis has not been
	// previously installed. Generate new password.
	// If users are accessing redis in a URL format, symbols can be illegal characters.
	if !ok {
		p, err := password.Generate(10, 4, 0, false /*noUpper*/, false /*allowRepeat*/)
		if err != nil {
			return "", errors.WithStack(err)
		}
		pass = []byte(p)
	}

	return base64.StdEncoding.EncodeToString(pass), nil
}

func GetRuntimeSecretData(
	ctx context.Context,
	kubeCtx string,
	ns string,
) (map[string][]byte, error) {
	secretData, err := getSecretData(ctx, kubeCtx, ns, RuntimeChartName)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return secretData, nil
}

func watchForContainerErrors(ctx context.Context, dp *DeployPlan) error {
	rc, err := RESTConfigFromDefaults(dp.DeployOptions.KubeContext)
	if err != nil {
		return errors.WithStack(err)
	}
	clientset, err := kubernetes.NewForConfig(rc)
	if err != nil {
		return errors.WithStack(err)
	}

	currentReleases, err := listReleases(ctx, dp.helmDriver, dp.DeployOptions.KubeContext, dp.DeployOptions.Namespace)
	if err != nil {
		return errorutil.CombinedError(err, errUnableToAccessHelmReleases)
	}

	appRelease := findRelease(currentReleases, dp.DeployOptions.App.ReleaseName)
	revision := 1
	if appRelease != nil {
		revision = appRelease.Version + 1
	}

	watcher, err := clientset.CoreV1().Pods(dp.appChartConfig.Namespace).Watch(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf(
			"app.kubernetes.io/name=%s,app.kubernetes.io/instance=%s,jetpack.io/revision=%d",
			dp.appChartConfig.Name,
			dp.appChartConfig.instanceName,
			revision,
		),
		Watch: true,
	})
	if err != nil {
		return errors.WithStack(err)
	}
	for event := range watcher.ResultChan() {
		if pod, ok := event.Object.(*corev1.Pod); ok {
			for _, containerStatus := range pod.Status.ContainerStatuses {
				if containerStatus.State.Waiting != nil {
					reason := containerStatus.State.Waiting.Reason
					if reason == "RunContainerError" || reason == "CrashLoopBackOff" {
						return errorutil.CombinedError(
							ErrPodContainerError,
							errorutil.NewUserErrorf(
								"[ERROR]: Application failed to start: %s\n",
								containerStatus.State.Waiting.Message,
							),
						)
					}
				}
			}
		}
	}
	return nil
}
