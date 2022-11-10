package command

import (
	"context"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/bep/debounce"
	"github.com/fatih/color"
	"github.com/pkg/errors"
	"github.com/radovskyb/watcher"
	ignore "github.com/sabhiram/go-gitignore"
	"github.com/spf13/cobra"
	"go.jetpack.io/launchpad/goutil"
	"go.jetpack.io/launchpad/launchpad"
	"go.jetpack.io/launchpad/padcli/command/jflags"
	"go.jetpack.io/launchpad/padcli/jetconfig"
	"go.jetpack.io/launchpad/pkg/jetlog"
	"golang.org/x/sync/errgroup"
	"helm.sh/helm/v3/pkg/strvals"
)

type devOptions struct {
	deployOptions
	embeddedBuildOptions
	publishOptions
}

func devCmd() *cobra.Command {

	opts := &devOptions{}

	devCmd := &cobra.Command{
		Use:     "dev [path]",
		Short:   "builds, deploys app, and forwards port",
		Args:    cobra.MaximumNArgs(1),
		Hidden:  true,
		PreRunE: validateDeployUptions(&opts.deployOptions),
		RunE: func(cmd *cobra.Command, args []string) error {
			absPath, err := projectDir(args)
			if err != nil {
				return errors.WithStack(err)
			}

			_, err = loadOrInitConfigFromFileSystem(cmd.Context(), cmd, args)
			if err != nil {
				return errors.WithStack(err)
			}

			ctx, err := cmdOpts.AuthProvider().Identify(cmd.Context())
			if err != nil {
				return errors.WithStack(err)
			}

			fsChanges, err := listenToFilesystemChanges(ctx, absPath)
			if err != nil {
				return errors.Wrap(err, "failed to listen to filesystem changes")
			}

			// Keep a buffer-one channel so that at-most one pending update is scheduled
			deployPending := bufferFilesystemEdits(fsChanges)

			// TODO savil. I should rethink how errors from goroutines
			// are collected and propagated
			errs, errGroupCTX := errgroup.WithContext(ctx)

			errs.Go(func() error {
				err := autoDeploy(errGroupCTX, deployPending, args, cmd, opts)
				return errors.Wrap(err, "autoDeploy failed")
			})

			// kick off a deploy without prompting the user.
			deployPending <- false

			return errs.Wait()
		},

		PostRun: func(cmd *cobra.Command, args []string) {
			if err := cleanupPreviousBuildsPostRun(cmd, args); err != nil {
				cmdOpts.ErrorLogger().CaptureException(err)
				return
			}
		},
	}

	registerDevFlags(devCmd, opts)
	return devCmd
}

func registerDevFlags(cmd *cobra.Command, opts *devOptions) {
	jflags.RegisterCommonFlags(cmd, cmdOpts)
	registerDeployFlags(cmd, &opts.deployOptions)
	registerEmbeddedBuildFlags(cmd, &opts.embeddedBuildOptions)
	registerPublishFlags(cmd, &opts.publishOptions)
}

func bufferFilesystemEdits(fsChanges chan bool) chan bool {
	deployPending := make(chan bool, 1)
	go func() {
		defer close(deployPending)
		for range fsChanges {
			// this select will go to default case if the deployPending channel is full
			select {
			case deployPending <- true:
			default:
			}
		}
	}()
	return deployPending
}

func autoDeploy(
	ctx context.Context,
	deployPending chan bool,
	cmdArgs []string,
	cmd *cobra.Command,
	opts *devOptions,
) error {

	logsAndPortFwdCancelFn := func() {}
	for shouldPromptUser := range deployPending {
		if shouldPromptUser {
			deploy := false
			err := survey.AskOne(&survey.Confirm{
				Message: "Code changes detected. Press enter to redeploy your application...",
				Default: true,
			}, &deploy, survey.WithIcons(func(is *survey.IconSet) {
				is.Question.Text = ""
			}))
			if errors.Is(err, terminal.InterruptErr) {
				break
			}
			if !deploy {
				continue
			}
		}

		jetCfg, err := loadOrInitConfigFromFileSystem(ctx, cmd, cmdArgs)
		if err != nil {
			logsAndPortFwdCancelFn()
			return errors.WithStack(err)
		}

		// If the selected cluster is a Jetpack-managed cluster, use its public hostname.
		cluster, err := cmdOpts.ClusterProvider().Get(ctx)
		if err != nil {
			logsAndPortFwdCancelFn()
			return errors.WithStack(err)
		}

		if err != nil {
			logsAndPortFwdCancelFn()
			return errors.WithStack(err)
		}
		// cancel any pending deploys
		logsAndPortFwdCancelFn()

		pad := launchpad.NewPad(cmdOpts.ErrorLogger())
		absPath, err := projectDir(cmdArgs)
		if err != nil {
			return errors.WithStack(err)
		}

		imageRepo := goutil.Coalesce(opts.ImageRepo, jetCfg.ImageRepository)
		repoConfig, err := cmdOpts.RepositoryProvider().Get(ctx, cluster, imageRepo)
		if err != nil {
			return errors.WithStack(err)
		}

		store, err := newEnvStore(ctx, cmd, cmdArgs, cmdOpts.EnvSecProvider(), jetCfg.Envsec.Provider)
		if err != nil {
			return errors.WithStack(err)
		}

		deployOutput, bpdErr := buildPublishAndDeploy(
			ctx,
			pad,
			cmd,
			jetCfg,
			&opts.embeddedBuildOptions,
			&opts.deployOptions,
			imageRepo,
			absPath,
			cluster,
			repoConfig,
			store,
		)

		var tailLogCtx context.Context
		tailLogCtx, logsAndPortFwdCancelFn = context.WithCancel(ctx)

		if bpdErr != nil {
			// NOTE: we should check that bpdError is a deploy error and not a build or publish error
			if errors.Is(bpdErr, launchpad.ErrPodContainerError) {
				logsAndPortFwdCancelFn()
				return bpdErr
			}
			// Deploy step failed. Tailing logs.
			if opts.deployOptions.execQualifiedSymbol != "" {
				if err := pad.TailLogsForAppExec(ctx, cluster.GetKubeContext(), deployOutput); err != nil {
					logsAndPortFwdCancelFn()
					return errors.Wrap(bpdErr, "failed to tail pod logs")
				}
			} else if err := pad.TailLogsOnErr(tailLogCtx, cluster.GetKubeContext(), deployOutput); err != nil {
				// at this point we do not want to continue with the rest of deployments
				logsAndPortFwdCancelFn()
				return errors.Wrap(bpdErr, "failed to tail pod logs")
			}
		} else {
			err := printUpSuccess(ctx, deployOutput, cluster)
			if err != nil {
				logsAndPortFwdCancelFn()
				return errors.Wrap(err, "failed to print namespace and app URL")
			}
			go func() {
				err := tailLogsAndPortForward(
					tailLogCtx,
					cluster.GetKubeContext(),
					jetCfg,
					opts,
					deployOutput,
					absPath,
				)
				if err != nil && !errors.Is(err, context.Canceled) {
					jetlog.Logger(ctx).Printf(color.RedString("ERROR: failed to tail logs and port forward, %v\n", err))
				}
			}()
		}
	}

	// at this point, deployPending has been closed:
	// cancel any deploys in progress
	logsAndPortFwdCancelFn()
	return nil
}

func tailLogsAndPortForward(
	ctx context.Context,
	kubeCtx string,
	jetCfg *jetconfig.Config,
	opts *devOptions,
	deployOut *launchpad.DeployOutput,
	absPath string,
) error {
	pad := launchpad.NewPad(cmdOpts.ErrorLogger())
	if opts.execQualifiedSymbol != "" {
		if err := pad.TailLogsForAppExec(ctx, kubeCtx, deployOut); err != nil {
			return errors.Wrap(err, "failed to tail pod logs")
		}
	} else {

		hasDepl, err := jetCfg.HasDeployment()
		if err != nil {
			return errors.WithStack(err)
		}

		if hasDepl {
			pfopts := &launchpad.PortForwardOptions{
				DeployOut: deployOut,
				KubeCtx:   kubeCtx,
				PodPort:   deployOut.AppPort(),
				Target:    launchpad.PortFwdTargetApp,
			}
			if err = pad.PortForward(ctx, pfopts); err != nil {
				return errors.Wrap(err, "failed to port forward for app")
			}
		}

		if deployOut.Releases[launchpad.RuntimeChartName] != nil {
			podPort, err := helmPodPortIfAny(opts.Runtime.SetValues)
			if err != nil {
				return errors.WithStack(err)
			}
			opts := &launchpad.PortForwardOptions{
				DeployOut: deployOut,
				KubeCtx:   kubeCtx,
				PodPort:   podPort,
				Target:    launchpad.PortFwdTargetRuntime,
			}
			if err = pad.PortForward(ctx, opts); err != nil {
				return errors.Wrap(err, "failed to port forward for runtime")
			}
		}

		if err := pad.TailLogs(ctx, kubeCtx, deployOut); err != nil {
			return errors.Wrap(err, "failed to tail pod logs")
		}
	}

	return nil
}

// helmPodPortIfAny tries to check helm.values.set for the podPort. Returns 0 if none found.
func helmPodPortIfAny(helmSetValues string) (int, error) {
	helmVals, err := strvals.Parse(helmSetValues)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to parse helm values: %s", helmSetValues)
	}
	customPodPort, ok := helmVals["podPort"]
	if !ok {
		return 0, nil
	}

	// The any value is of type int64, so first cast to that.
	// Then truncate the size to int. On 32-bit machines this is
	// destructive, but for port numbers this is almost certainly safe.
	return int(customPodPort.(int64)), nil
}

func listenToFilesystemChanges(ctx context.Context, absPath string) (chan bool, error) {
	fsWatcher := watcher.New()
	// SetMaxEvents to 1 to allow at most 1 event's to be received
	// on the Event channel per watching cycle.
	// If SetMaxEvents is not set, the default is to send all events.
	fsWatcher.SetMaxEvents(1)

	// Only notify certain events.
	fsWatcher.FilterOps(watcher.Write, watcher.Create, watcher.Remove, watcher.Rename, watcher.Move)

	fsWatcher.IgnoreHiddenFiles(true)

	// Add directories in .dockerignore to the ignore list
	if err := syncDir(ctx, fsWatcher, absPath); err != nil {
		return nil, errors.WithStack(err)
	}

	// Watch path recursively for changes.
	if err := fsWatcher.AddRecursive(absPath); err != nil {
		return nil, errors.WithStack(err)
	}

	fsChanges := make(chan bool)
	go func() {
		defer func() {
			fsWatcher.Close()
			close(fsChanges)
		}()

		debouncer := debounce.New(1000 * time.Millisecond) // debounce for 1 second
		for {
			select {
			case <-ctx.Done():
				return
			case err := <-fsWatcher.Error:
				jetlog.Logger(ctx).Printf("ERROR: file watcher errored with: %v\n", err)
				// carry on
			case <-fsWatcher.Event:
				debouncer(func() { fsChanges <- true })
			}
		}
	}()

	// Start the watching process - it'll check for changes every 1000ms.
	go func() {
		if err := fsWatcher.Start(time.Millisecond * 1000); err != nil {
			jetlog.Logger(ctx).Printf("ERROR: unable to start file watcher: %v\n", err)
		}
	}()

	return fsChanges, nil
}

func syncDir(ctx context.Context, w *watcher.Watcher, dir string) error {
	var filterFunc = func(string) bool { return false }
	// recursively find directories because fsnotify's watcher doesn't recursively check
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return errors.WithStack(err)
		}
		if d.IsDir() && isHiddenDir(d) {
			return filepath.SkipDir
		}

		filename := filepath.Base(path)
		if filename == ".dockerignore" {
			ignoreMatcher, err := ignore.CompileIgnoreFile(path)
			if err != nil {
				return errors.WithStack(err)
			}
			filterFunc = ignoreMatcher.MatchesPath
		}
		if filterFunc(path) {
			// if ignore path failed, it should continue.
			if err := w.Ignore(path); err != nil {
				// Simply do nothing for now and print it out?
				jetlog.Logger(ctx).Printf("[ERROR]: file watcher failed to ignore file: %s", path)
			}
			if d.IsDir() {
				return filepath.SkipDir
			}
		}
		return nil
	})
	return errors.Wrapf(err, "filePath walk failed for directory %s", dir)
}

func isHiddenDir(d fs.DirEntry) bool {
	// Ignore hidden directories like .git
	//
	// NOTE: This works for *nix systems but not windows
	return d.IsDir() && strings.HasPrefix(d.Name(), ".")
}
