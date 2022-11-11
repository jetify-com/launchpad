package command

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/fatih/color"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"go.jetpack.io/envsec"
	"go.jetpack.io/launchpad/goutil"
	"go.jetpack.io/launchpad/launchpad"
	"go.jetpack.io/launchpad/padcli/helm"
	"go.jetpack.io/launchpad/padcli/jetconfig"
	"go.jetpack.io/launchpad/padcli/provider"
	"go.jetpack.io/launchpad/pkg/jetlog"
)

type HelmOptions struct {
	ChartLocation string
	Name          string
	ValueFiles    []string
	SetValues     string
}

type deployOptions struct {
	App     HelmOptions
	Runtime HelmOptions

	envOverrideFile     string
	execQualifiedSymbol string
	SecretFilePaths     []string

	Namespace string

	ReinstallOnHelmUpgradeError bool
}

func makeDeployOptions(
	ctx context.Context,
	cmd *cobra.Command,
	jetCfg *jetconfig.Config,
	publishOutput *launchpad.PublishOutput,
	buildOutput *launchpad.BuildOutput,
	opts *deployOptions,
	modulePath string,
	cluster provider.Cluster,
	store envsec.Store,
) (*launchpad.DeployOptions, error) {

	ns, err := cmdOpts.NamespaceProvider().Get(ctx, opts.Namespace, cluster.GetKubeContext(), cmdOpts.RootFlags().Env())
	if err != nil {
		return nil, err
	}

	hvc := helm.NewValueComputer(
		cmdOpts.RootFlags().Env(),
		ns,
		opts.execQualifiedSymbol,
		helm.NewImageProvider(
			buildOutput.Image.String(),
			publishOutput.PublishedImages(),
			publishOutput.RegistryHost(),
		),
		jetCfg,
		cluster,
	)

	if err := cmdOpts.Hooks().PreHelmValuesCompute(ctx, cmdOpts, hvc); err != nil {
		return nil, err
	}

	l := jetlog.Logger(ctx)
	boldSprint := color.New(color.Bold).Sprint
	fmt.Fprintln(l)
	fmt.Fprintln(l, "\tProject:     "+boldSprint(jetCfg.GetProjectName()))
	fmt.Fprintln(l, "\tNamespace:   "+boldSprint(ns))
	fmt.Fprintln(l, "\tCluster:     "+boldSprint(cluster.GetKubeContext()))
	fmt.Fprintln(l, "\tEnvironment: "+boldSprint(cmdOpts.RootFlags().Env().ToLower()))

	if err := hvc.Compute(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to compute helm values")
	}
	appValues, err := cmdOpts.Hooks().PostAppChartValuesCompute(
		ctx,
		cmdOpts,
		hvc,
	)
	if err != nil {
		return nil, err
	}
	runtimeValues, err := cmdOpts.Hooks().PostRuntimeChartValuesCompute(
		ctx,
		cmdOpts,
		hvc,
	)
	if err != nil {
		return nil, err
	}
	runtimeValues, err = helm.MergeValues(
		runtimeValues,
		[]string{}, // values files
		opts.Runtime.SetValues,
	)
	if err != nil {
		return nil, errors.Wrap(err, "Failed merging --helm.runtime.set values")
	}

	var runtimeHelm *launchpad.HelmOptions
	if len(runtimeValues) != 0 {
		runtimeHelm = &launchpad.HelmOptions{
			ChartLocation: opts.Runtime.ChartLocation,
			Values:        runtimeValues,
			// Not ideal, but better than failing. We need to fine tune.
			Timeout: 5 * time.Minute,
		}
	}

	// if --env-override flag was explicitly set, then add values to helmOptions
	appSecrets := map[string]string{}
	if opts.envOverrideFile != "" {
		appSecrets, err = readEnvVariables(
			modulePath,
			opts.envOverrideFile,
			true, // encodedValues
		)
		if err != nil {
			return nil, errors.WithStack(err)
		}
	}

	if cmd.Flags().Changed(mountSecretFileFlag) && cmd.Flags().Changed(mountSecretFilesFlag) {
		return nil, errors.Errorf("Incompatible command line opts '%v' and '%v' : Prefer specifying a list of secret files using '%v'", mountSecretFileFlag, mountSecretFilesFlag, mountSecretFilesFlag)
	}

	appValues["secrets"] = appSecrets
	appValues, err = helm.MergeValues(
		appValues,
		lo.Map(
			opts.App.ValueFiles, func(f string, _ int) string {
				return filepath.Join(modulePath, f)
			}),
		opts.App.SetValues,
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to merge app values")
	}

	remoteEnvVars, err := getRemoteEnvVars(ctx, jetCfg, store)
	if err != nil {
		return nil, err
	}

	return &launchpad.DeployOptions{
		App: &launchpad.HelmOptions{
			ChartLocation: opts.App.ChartLocation,
			InstanceName:  getInstanceName(jetCfg),
			ReleaseName:   getReleaseName(jetCfg),
			Values:        appValues,
			Timeout:       lo.Ternary(len(jetCfg.Jobs()) > 0, 5*time.Minute, 0),
		},
		Environment:                 cmdOpts.RootFlags().Env().String(),
		ExternalCharts:              jetconfigHelmToChartConfig(jetCfg, ns),
		JetCfg:                      jetCfg,
		IsLocalCluster:              cluster.IsLocal(),
		KubeContext:                 cluster.GetKubeContext(),
		LifecycleHook:               cmdOpts.Hooks().Deploy,
		Namespace:                   ns,
		RemoteEnvVars:               remoteEnvVars,
		Runtime:                     runtimeHelm,
		SecretFilePaths:             opts.SecretFilePaths,
		ReinstallOnHelmUpgradeError: opts.ReinstallOnHelmUpgradeError,
	}, nil
}

func getReleaseName(jetCfg *jetconfig.Config) string {
	return helm.ToValidName(goutil.Coalesce(jetCfg.GetProjectID(), jetCfg.GetProjectName()))
}

func getInstanceName(jetCfg *jetconfig.Config) string {
	return helm.ToValidName(jetCfg.GetInstanceName())
}
