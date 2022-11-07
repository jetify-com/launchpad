package command

import (
	"context"

	"github.com/pkg/errors"
	"github.com/samber/lo"

	"go.jetpack.io/envsec"
	"go.jetpack.io/launchpad/launchpad"
	"go.jetpack.io/launchpad/padcli/helm"
	"go.jetpack.io/launchpad/padcli/jetconfig"
)

func jetconfigHelmToChartConfig(
	jetCfg *jetconfig.Config,
	ns string, // Remove this since all charts are in the same namespace.
) []*launchpad.ChartConfig {
	return lo.Map(
		jetCfg.HelmCharts(),
		func(hc jetconfig.HelmChart, _ int) *launchpad.ChartConfig {
			return &launchpad.ChartConfig{
				Repo:      hc.GetRepo(),
				Name:      hc.GetName(),
				Namespace: ns,
				Release:   helm.ToValidName(hc.GetName() + "-" + jetCfg.GetProjectSlug()),
				Wait:      false, // we have no idea what's in here. It might be slow
			}
		},
	)
}

func getRemoteEnvVars(
	ctx context.Context,
	jetCfg *jetconfig.Config,
	store envsec.Store,
) (map[string]string, error) {

	envVars := map[string]string{}
	if store == nil {
		return envVars, nil
	}
	// load secrets from parameter store
	envID, err := cmdOpts.EnvSecProvider().NewEnvId(ctx, jetCfg.GetProjectID(), cmdOpts.RootFlags().Env().String())
	if err != nil {
		return nil, errors.WithStack(err)
	}
	storeEnvVars, err := store.List(ctx, *envID)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	for _, envVar := range storeEnvVars {
		envVars[envVar.Name] = envVar.Value
	}
	return envVars, nil
}
