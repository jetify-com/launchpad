package launchpad

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	"go.jetpack.io/launchpad/goutil/errorutil"
	"go.jetpack.io/launchpad/pkg/buildstamp"
	"go.jetpack.io/launchpad/pkg/jetlog"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/repo"
	"helm.sh/helm/v3/pkg/storage/driver"
)

const (
	repoURL         = "https://releases.jetpack.io/charts/v0"
	noChangesPrefix = "Looks like there are no changes for"
	spinnerMessage  = "\tWaiting for deployment to be ready "
)

const defaultHelmTimeout = time.Minute

var helmOutputPrefixes = []string{"Deployment is not ready:", "StatefulSet is not ready:"}

func applyHelmCharts(
	ctx context.Context,
	plan *DeployPlan,
) (map[string]*release.Release, error) {
	settings := newSettings(plan.DeployOptions.KubeContext)

	releases := make(map[string]*release.Release)
	currentReleases, err := listReleases(ctx, plan.helmDriver, plan.DeployOptions.KubeContext, plan.DeployOptions.Namespace)
	if err != nil {
		return nil, errorutil.CombinedError(err, errUnableToAccessHelmReleases)
	}

	for _, cc := range plan.Charts() {
		c, err := actionConfig(ctx, plan.helmDriver, cc.Namespace, settings)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		r := findRelease(currentReleases, cc.Release)
		if r != nil {
			releases[cc.Name], err = upgradeHelmChart(ctx, cc, settings, c)

			if plan.DeployOptions.ReinstallOnHelmUpgradeError {
				releases[cc.Name], err = reinstallHelmChart(ctx, cc.Release, cc, settings, c)
				if err != nil {
					return nil, errors.WithStack(err)
				}
			}

			if errors.Is(err, driver.ErrNoDeployedReleases) {
				return nil, errorutil.CombinedError(err, errUserUpgradeFail)
			} else if err != nil {
				return nil, errors.WithStack(err)
			}
		} else {
			// Try to see if the old release is still using app name as the release name
			r := findRelease(currentReleases, cc.instanceName)
			if r != nil {
				// We will automatically down the old release and up a new release.
				// Since the old release is using app name as the release name.
				jetlog.Logger(ctx).IndentedPrintln("Detected old install by the project name. Changing to install by project ID.")
				releases[cc.Name], err = reinstallHelmChart(ctx, cc.instanceName, cc, settings, c)
				if err != nil {
					return nil, errors.WithStack(err)
				}
			} else {
				releases[cc.Name], err = installHelmChart(ctx, cc, settings, c)
				if err != nil {
					return releases, errors.Wrap(err, "Error installing helm chart")
				}
			}
		}
	}

	for _, chart := range plan.DeployOptions.ExternalCharts {
		c, err := actionConfig(ctx, plan.helmDriver, chart.Namespace, settings)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		r := findRelease(currentReleases, chart.Release)
		if r != nil {
			_, err = upgradeHelmChart(ctx, chart, settings, c)
			if err != nil {
				return nil, err
			}
		} else {
			_, err = installHelmChart(ctx, chart, settings, c)
			if err != nil {
				return nil, err
			}
		}
	}

	return releases, nil
}

func installHelmChart(
	ctx context.Context,
	cc *ChartConfig,
	settings *cli.EnvSettings,
	config *action.Configuration,
) (*release.Release, error) {
	install := action.NewInstall(config)

	chart, err := getChart(cc, settings, install.ChartPathOptions)
	if err != nil {
		return nil, errors.Wrap(err, "Error loading chart")
	}

	install.Namespace = cc.Namespace
	install.ReleaseName = cc.Release
	// For Jetpack-managed clusters, namespace is created (and permissions are set) by InitNamespace, so this
	// will be a no-op. For other clusters, we set to true in case the namespace doesn't exist yet.
	install.CreateNamespace = true
	install.Wait = cc.Wait
	install.Timeout = cc.Timeout

	jetlog.Logger(ctx).BoldPrintf("Installing %s...\n", cc.HumanName())
	rel, err := install.RunWithContext(ctx, chart, cc.values)
	if err != nil {
		return rel, errors.Wrap(err, "Error installing helm chart")
	}
	jetlog.Logger(ctx).BoldPrintf("Successfully installed %s\n", cc.HumanName())
	return rel, nil
}

func upgradeHelmChart(
	ctx context.Context,
	cc *ChartConfig,
	settings *cli.EnvSettings,
	config *action.Configuration,
) (*release.Release, error) {
	upgrade := action.NewUpgrade(config)

	chart, err := getChart(cc, settings, upgrade.ChartPathOptions)
	if err != nil {
		return nil, errors.Wrap(err, "Error loading chart")
	}

	upgrade.Namespace = cc.Namespace
	upgrade.Wait = cc.Wait
	upgrade.Timeout = cc.Timeout
	upgrade.MaxHistory = 10 // 10 is the CLI default

	jetlog.Logger(ctx).BoldPrintf("Upgrading %s...\n", cc.HumanName())
	rel, err := upgrade.RunWithContext(ctx, cc.Release, chart, cc.values)
	if err != nil {
		return rel, errors.Wrap(err, "Error upgrading helm chart")
	}
	jetlog.Logger(ctx).BoldPrintf("Successfully upgraded %s\n", cc.HumanName())
	return rel, nil
}

func uninstallChart(
	ctx context.Context,
	downPlan *downPlan,
) error {
	settings := newSettings(downPlan.downOptions.KubeContext)
	config := &action.Configuration{}
	err := config.Init(
		settings.RESTClientGetter(),
		downPlan.namespace,
		downPlan.helmDriver,
		func(format string, v ...any) {
			jetlog.Logger(ctx).IndentedPrintf(format+"\n", v...)
		})
	if err != nil {
		return errors.Wrap(err, "Error initializing chart config")
	}

	for _, rel := range downPlan.releases {

		uninstall := action.NewUninstall(config)
		uninstall.Wait = true

		jetlog.Logger(ctx).BoldPrintf("Uninstalling %s...\n", rel.InstanceName)
		_, err = uninstall.Run(rel.ReleaseName)
		if err != nil {
			return errors.Wrapf(err, "failed to uninstall release-name: %s", rel.InstanceName)
		}

		jetlog.Logger(ctx).BoldPrintf("Successfully uninstalled %s\n", rel.InstanceName)
	}

	for _, chart := range downPlan.downOptions.ExternalCharts {

		uninstall := action.NewUninstall(config)

		jetlog.Logger(ctx).BoldPrintf("Uninstalling %s...\n", chart.Release)
		_, err = uninstall.Run(chart.Release)
		if err != nil {
			return errors.Wrapf(err, "failed to uninstall release-name: %s", chart.Release)
		}

		jetlog.Logger(ctx).BoldPrintf("Successfully uninstalled %s\n", chart.Release)
	}

	return nil
}

func getChart(
	cc *ChartConfig,
	settings *cli.EnvSettings,
	cpo action.ChartPathOptions,
) (*chart.Chart, error) {
	if cc.chartLocation != "" {
		c, err := loader.Load(cc.chartLocation)
		return c, errors.Wrap(err, "Error loading chart")
	}

	var err error
	chartURL := fmt.Sprintf("%s/%s-%s.tgz", repoURL, cc.Name, cc.chartVersion)
	if cc.Repo != "" {
		chartURL, err = repo.FindChartInRepoURL(
			cc.Repo, cc.Name, "", "", "", "", getter.All(settings))
		if err != nil {
			return nil, errors.Wrap(err, "Error finding chart in repo")
		}
	}

	cp, err := cpo.LocateChart(chartURL, settings)

	buildstmp := buildstamp.Get()
	if err != nil && strings.Contains(err.Error(), "failed to fetch") && buildstmp.IsDevBinary() {
		// This can happen in development because referenced version is dirty and
		// the helm chart has not been released. In this case, we use the latest chart version.
		cp, err = getLatestChart(cc, settings, cpo)
	}

	if err != nil {
		return nil, errors.Wrap(err, "Error locating chart")
	}
	c, err := loader.Load(cp)

	return c, errors.Wrap(err, "Error loading chart")
}

func getLatestChart(
	cc *ChartConfig,
	settings *cli.EnvSettings,
	cpo action.ChartPathOptions,
) (string, error) {
	r, err := repo.NewChartRepository(
		&repo.Entry{Name: chartRepoName, URL: repoURL},
		getter.All(settings),
	)

	if err != nil {
		return "", errors.WithStack(err)
	}

	pathToIndexFile, err := r.DownloadIndexFile()

	if err != nil {
		return "", errors.WithStack(err)
	}

	idx, err := repo.LoadIndexFile(pathToIndexFile)
	if err != nil {
		return "", errors.WithStack(err)
	}

	if len(idx.Entries[RuntimeChartName]) == 0 {
		return "", errNoValidChartVersions
	}

	idx.SortEntries()

	return cpo.LocateChart(
		fmt.Sprintf(
			"%s/%s-%s.tgz",
			repoURL,
			cc.Name,
			idx.Entries[RuntimeChartName][0].Version,
		),
		settings,
	)
}

func getRelease(
	ctx context.Context,
	helmDriver string,
	cc *ChartConfig,
	settings *cli.EnvSettings,
) (*release.Release, error) {

	cfg, err := actionConfig(ctx, helmDriver, cc.Namespace, settings)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return action.NewGet(cfg).Run(cc.Release)
}

func listReleases(
	ctx context.Context,
	helmDriver string,
	KubeContext string,
	namespace string,
) ([]*release.Release, error) {
	s := newSettings(KubeContext)
	cfg, err := actionConfig(ctx, helmDriver, namespace, s)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return action.NewList(cfg).Run()
}

func findRelease(releases []*release.Release, releaseName string) *release.Release {
	for _, r := range releases {
		if r.Name == releaseName {
			return r
		}
	}
	return nil
}

func getValues(
	ctx context.Context,
	helmDriver string,
	cc *ChartConfig,
	settings *cli.EnvSettings,
) (map[string]any, error) {
	cfg, err := actionConfig(ctx, helmDriver, cc.Namespace, settings)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return action.NewGetValues(cfg).Run(cc.Release)
}

func actionConfig(
	ctx context.Context,
	helmDriver,
	namespace string,
	settings *cli.EnvSettings,
) (*action.Configuration, error) {

	cfg := &action.Configuration{}
	if err := cfg.Init(
		settings.RESTClientGetter(),
		namespace,
		helmDriver,
		func(format string, v ...any) {
			if !strings.HasPrefix(format, noChangesPrefix) {
				jetlog.Logger(ctx).WithSpinnerPrintf(helmOutputPrefixes, spinnerMessage, format+"\n", v...)
			}
		}); err != nil {
		return nil, errors.Wrap(err, "Error initializing chart config")
	}

	return cfg, nil
}

func newSettings(kubeCtx string) *cli.EnvSettings {
	settings := cli.New()
	if kubeCtx != "" {
		settings.KubeContext = kubeCtx
	}
	return settings
}

func reinstallHelmChart(
	ctx context.Context,
	uninstallReleaseName string,
	cc *ChartConfig,
	settings *cli.EnvSettings,
	config *action.Configuration,
) (*release.Release, error) {
	jetlog.Logger(ctx).BoldPrintf("Could not upgrade. Reinstalling...\n")
	uninstall := action.NewUninstall(config)
	uninstall.Wait = cc.Wait
	_, err := uninstall.Run(uninstallReleaseName)
	if err != nil {
		return nil, errorutil.CombinedError(err, errUserReinstallFail)
	}
	return installHelmChart(ctx, cc, settings, config)
}
