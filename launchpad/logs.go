package launchpad

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"text/template"
	"time"

	"github.com/fatih/color"
	"github.com/pkg/errors"
	"github.com/stern/stern/stern"
	"go.jetpack.io/launchpad/pkg/jetlog"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
)

var matchAllRegex = regexp.MustCompile(".*")

const logTemplate = "{{color .PodColor .PodName}} {{color .ContainerColor .ContainerName}} {{.Message}}\n"

func tailLogs(
	ctx context.Context,
	kubeCtx string,
	ns string,
	name string,
	revision int,
	containerState stern.ContainerState,
) error {
	err := tailLogsForApp(
		ctx,
		kubeCtx,
		ns,
		name,
		revision,
		containerState,
	)
	if err != nil {
		return errors.WithStack(err)
	}

	err = tailLogsForRuntime(
		ctx,
		kubeCtx,
		ns,
		name,
		containerState,
	)
	return errors.WithStack(err)
}

func tailLogsForExec(
	ctx context.Context,
	kubeCtx string,
	ns string,
	name string,
	revision int,
) error {
	err := tailLogsForAppExec(
		ctx,
		kubeCtx,
		ns,
		name,
		revision)
	if err != nil {
		return errors.WithStack(err)
	}

	err = tailLogsForRuntime(
		ctx,
		kubeCtx,
		ns,
		name,
		stern.RUNNING,
	)
	return errors.WithStack(err)
}

func tailLogsForApp(
	ctx context.Context,
	kubeCtx string,
	ns string,
	name string,
	revision int,
	containerState stern.ContainerState,
) error {
	podLabels := getPodLabelForApp(name, revision)
	includeRegexp := []*regexp.Regexp{}
	err := tailLogsImpl(
		ctx,
		kubeCtx,
		ns,
		podLabels,
		includeRegexp,
		time.Hour,
		[]stern.ContainerState{containerState},
	)
	return errors.WithStack(err)
}

func tailLogsForAppExec(
	ctx context.Context,
	kubeCtx string,
	ns string,
	name string,
	revision int,
) error {
	podLabels := fmt.Sprintf(
		"app.kubernetes.io/name=%s,app.kubernetes.io/instance=%s,jetpack.io/revision=%d",
		fmt.Sprintf("%s-sdk-exec", AppChartName),
		name,
		revision,
	)
	includeRegexp := []*regexp.Regexp{}
	err := tailLogsImpl(
		ctx,
		kubeCtx,
		ns,
		podLabels,
		includeRegexp,
		time.Hour,
		[]stern.ContainerState{stern.RUNNING, stern.TERMINATED},
	)
	return errors.WithStack(err)
}

func tailLogsForRuntime(
	ctx context.Context,
	kubeCtx string,
	ns string,
	appName string,
	containerState stern.ContainerState,
) error {
	podLabels := getPodLabelForRuntime()
	includeRegexp := []*regexp.Regexp{regexp.MustCompile(appNamePrefixForLog(appName))}
	err := tailLogsImpl(
		ctx,
		kubeCtx,
		ns,
		podLabels,
		includeRegexp,
		time.Second,
		[]stern.ContainerState{containerState},
	)
	return errors.WithStack(err)
}

func getPodLabelForApp(name string, revision int) string {
	return fmt.Sprintf(
		"app.kubernetes.io/name=%s,app.kubernetes.io/instance=%s,jetpack.io/revision=%d",
		AppChartName,
		name,
		revision,
	)
}

func getPodLabelForRuntime() string {
	return fmt.Sprintf(
		"app.kubernetes.io/name=%s",
		RuntimeChartName,
	)
}

func tailLogsImpl(
	ctx context.Context,
	kubeCtx string,
	ns string,
	podLabels string,
	includeRegexp []*regexp.Regexp,
	since time.Duration,
	containerStates []stern.ContainerState, // stern.RUNNING, stern.TERMINATED
) error {

	labelSelector, err := labels.Parse(podLabels)
	if err != nil {
		return errors.Wrapf(err, "failed to parse selector (%s) as label selector", podLabels)
	}

	logTemplate, err := makeTemplate()
	if err != nil {
		return errors.Wrap(err, "failed to make template")
	}

	sternCfg := &stern.Config{
		ContainerQuery:  matchAllRegex,
		ContainerStates: containerStates,
		ContextName:     kubeCtx,
		FieldSelector:   fields.Everything(),
		Follow:          true,
		Include:         includeRegexp,
		// https://kubernetes.io/docs/concepts/workloads/pods/init-containers/
		InitContainers:      true,
		EphemeralContainers: true,
		LabelSelector:       labelSelector,
		// I don't really know why the time-location is needed, but if I set it
		// to nil, then the log-printing doesn't work. time.Local is what stern's
		// CLI uses.
		Location:   time.Local,
		Namespaces: []string{ns},
		PodQuery:   matchAllRegex,
		Since:      since,
		Template:   logTemplate,

		Out:    jetlog.Logger(ctx),
		ErrOut: jetlog.Logger(ctx),
	}
	go func() {
		err := stern.Run(ctx, sternCfg)
		if err != nil {
			jetlog.Logger(ctx).Printf("ERROR: stern.Run returned error %v\n", err)
			return
		}
	}()
	return nil
}

// inspired by code from:
// https://github.com/stern/stern/blob/master/cmd/cmd.go#L220-L253
func makeTemplate() (*template.Template, error) {

	funcs := map[string]any{
		"json": func(in any) (string, error) {
			b, err := json.Marshal(in)
			if err != nil {
				return "", err
			}
			return string(b), nil
		},
		"parseJSON": func(text string) (map[string]any, error) {
			obj := make(map[string]any)
			if err := json.Unmarshal([]byte(text), &obj); err != nil {
				return obj, err
			}
			return obj, nil
		},
		"color": func(color color.Color, text string) string {
			return color.SprintFunc()(text)
		},
	}
	t, err := template.New("log").Funcs(funcs).Parse(logTemplate)
	return t, errors.Wrap(err, "unable to parse template")
}

// appNamePrefixForLog is used by both the runtime and the CLI's launchpad library
// to mark a log from the runtime as being relevant to a particular app.
func appNamePrefixForLog(appName string) string {
	return fmt.Sprintf("app: %s", appName)
}
