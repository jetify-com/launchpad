package helm

import (
	"context"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	"github.com/samber/lo"

	"go.jetpack.io/launchpad/padcli/jetconfig"
	"go.jetpack.io/launchpad/padcli/provider"
	"go.jetpack.io/launchpad/proto/api"
)

// type cluster interface {
// 	GetHostname() string
// 	GetIsPrivate() bool
// 	GetName() string
// 	IsLocal() bool
// 	IsJetpackManaged() bool
// 	IsRemoteUnmanaged() bool
// }

// ValueComputer transforms jetpack CLI inputs into helm values.
// Right now we mostly copy paste the logic, but the idea is to have individual
// modules that compute sections of the values. e.g. ambassadorModule, cronjobModule, etc.
type ValueComputer struct {
	appValues     map[string]any
	runtimeValues map[string]any

	env                 api.Environment
	namespace           string // The final namespace to be used
	execQualifiedSymbol string // Used by jetpack dev <path/to/project> --exec <symbol>
	imageProvider       *ImageProvider
	jetCfg              *jetconfig.Config // consider interface
	cluster             provider.Cluster
}

func NewValueComputer(
	env api.Environment,
	namespace string,
	execQualifiedSymbol string,
	imageProvider *ImageProvider,
	jetCfg *jetconfig.Config, // consider interface
	c provider.Cluster,
) *ValueComputer {
	return &ValueComputer{
		env:                 env,
		namespace:           namespace,
		execQualifiedSymbol: execQualifiedSymbol,
		imageProvider:       imageProvider,
		jetCfg:              jetCfg,
		cluster:             c,
	}
}

func (hvc *ValueComputer) AppValues() map[string]any {
	return hvc.appValues
}

func (hvc *ValueComputer) RuntimeValues() map[string]any {
	return hvc.runtimeValues
}

func (hvc *ValueComputer) ImageProvider() *ImageProvider {
	return hvc.imageProvider
}

// Compute converts command options to helm values.
func (hvc *ValueComputer) Compute(ctx context.Context) error {
	hvc.appValues = map[string]any{}
	hvc.runtimeValues = map[string]any{}

	websvc, err := hvc.jetCfg.WebService()
	if err != nil {
		return errors.WithStack(err)
	}

	if hvc.cluster.IsJetpackManaged() {
		SetNestedField(hvc.appValues, "jetpack", "clusterHostname", hvc.cluster.GetHostname())
		SetNestedField(hvc.runtimeValues, "jetpack", "clusterHostname", hvc.cluster.GetHostname())

		if websvc != nil {
			hostname, err := hvc.ComputeHostname(ctx)
			if err != nil {
				return errors.WithStack(err)
			}
			SetNestedField(hvc.appValues, "ambassador", "hostname", hostname)

			url, err := websvc.GetURL()
			if err != nil {
				return errors.Wrap(err, "unable to get web service url")
			}
			if url.Path != "" {
				SetNestedField(hvc.appValues, "ambassador", "urlPrefix", url.Path)
			}
		}
	}

	if hvc.execQualifiedSymbol != "" {
		setSDKExecValues(hvc.appValues, hvc.execQualifiedSymbol)
	}

	SetNestedField(hvc.appValues, "jetpack", "cronjobs", lo.Map(
		hvc.jetCfg.Cronjobs(),
		func(cj jetconfig.Cron, _ int) any {
			return map[string]any{
				"name":     ToValidName(cj.GetUniqueName()),
				"schedule": cj.GetSchedule(),
				"image":    hvc.imageProvider.get(hvc.cluster, cj.GetImage()),
				"command":  cj.GetCommand(),
				"resources": map[string]any{
					"requests": map[string]any{
						"cpu":    cj.GetInstanceType().Compute(),
						"memory": cj.GetInstanceType().Memory(),
					},
				},
			}
		},
	))

	SetNestedField(hvc.appValues, "jetpack", "jobs", lo.Map(
		hvc.jetCfg.Jobs(),
		func(j jetconfig.Job, _ int) any {
			return map[string]any{
				"name":    ToValidName(j.GetUniqueName()),
				"image":   hvc.imageProvider.get(hvc.cluster, j.GetImage()),
				"command": j.GetCommand(),
				"resources": map[string]any{
					"requests": map[string]any{
						"cpu":    j.GetInstanceType().Compute(),
						"memory": j.GetInstanceType().Memory(),
					},
				},
			}
		},
	))

	SetNestedField(hvc.appValues, "jetpack", "projectId", hvc.jetCfg.GetProjectID())

	// A bit lame but required because technically can be nil.
	if websvc != nil {
		setNestedFieldPath(
			hvc.appValues,
			[]string{"resources", "requests", "cpu"},
			websvc.GetInstanceType().Compute(),
		)

		setNestedFieldPath(
			hvc.appValues,
			[]string{"resources", "requests", "memory"},
			websvc.GetInstanceType().Memory(),
		)

		hvc.appValues["podPort"] = websvc.GetPort()
	} else {
		// This logic will change once we allow multiple services
		hvc.appValues["replicaCount"] = 0
		SetNestedField(hvc.appValues, "serviceAccount", "create", false)
	}

	if hvc.cluster.IsLocal() {
		SetNestedField(hvc.appValues, "service", "type", "NodePort")
	}

	if websvc != nil {
		repo, tag := hvc.imageProvider.getSplit(hvc.cluster, websvc.GetImage())
		hvc.appValues["image"] = map[string]any{
			"repository": repo,
			"tag":        tag,
		}
	}

	return nil
}

func (hvc *ValueComputer) ComputeHostname(ctx context.Context) (string, error) {
	appFragment := ""
	// Hack so we don't break old Smarthop URLs.
	if hvc.cluster.GetHostname() == "smarthop.jetpack.dev" {
		appFragment = "-app"
	}
	websvc, err := hvc.jetCfg.WebService()
	if err != nil {
		return "", errors.WithStack(err)
	}
	hostname := websvc.GetName() + appFragment + "-" + hvc.namespace + "." + hvc.cluster.GetHostname()
	url, err := websvc.GetURL()
	if err != nil {
		return "", errors.Wrap(err, "unable to get web service url")
	}
	if url.Host != "" {
		hostname = url.Host
	}
	return hostname, nil
}

var alphanumericRegex = regexp.MustCompile(`[^a-zA-Z0-9]`)

func ToValidName(name string) string {
	name = alphanumericRegex.ReplaceAllString(name, "-")
	return strings.ToLower(name)
}

func (hvc *ValueComputer) RequiresCustomHost(ctx context.Context) (bool, error) {
	if websvc, err := hvc.jetCfg.WebService(); websvc == nil {
		return false, err
	}
	h, err := hvc.ComputeHostname(ctx)
	if err != nil {
		return false, err
	}
	return !strings.HasSuffix(h, "."+hvc.cluster.GetHostname()), nil
}

func (hvc *ValueComputer) Environment() api.Environment {
	return hvc.env
}

func (hvc *ValueComputer) Namespace() string {
	return hvc.namespace
}

func (hvc *ValueComputer) Cluster() provider.Cluster {
	return hvc.cluster
}
