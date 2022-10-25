package launchpad

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat"
	"github.com/fatih/color"
	"github.com/imdario/mergo"
	"github.com/pkg/errors"
	"go.jetpack.io/launchpad/goutil"
	"go.jetpack.io/launchpad/goutil/errorutil"
	"go.jetpack.io/launchpad/pkg/jetlog"
)

var green = color.New(color.FgGreen)
var yellow = color.New(color.FgYellow)

type LocalOptions struct {
	BuildOut            *BuildOutput
	SdkCmd              string
	ExecQualifiedSymbol string
	LocalEnvVars        map[string]string
	RemoteEnvVars       map[string]string
}

func (p *Pad) CreateAndStartContainerInLocalMode(ctx context.Context, opts *LocalOptions) (err error) {
	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return errorutil.CombinedError(err, errUserNoDockerClient)
	}

	sdkCmdSlice := []string{}
	cmd := opts.SdkCmd
	if opts.ExecQualifiedSymbol != "" {
		cmd = fmt.Sprintf("exec-cronjob --qualified-symbol=%s --match-suffix", opts.ExecQualifiedSymbol)
	}

	if cmd != "" {
		sdkCmdSlice = append(sdkCmdSlice, "jetpack-sdk")
		sdkCmdSlice = append(sdkCmdSlice, strings.Split(cmd, " ")...)
	}

	portLocalhostToContainers := map[string]string{
		"8080": "8080",
		// NOTE: we need to use 8085 here, because 8081 is taken by toast :/
		// TODO: We should also make the container grpc-port a parameter, instead
		// of harcoding it here.
		"8085": "8081",
	}
	portBindings := map[nat.Port][]nat.PortBinding{}
	for portLocalhost, portContainer := range portLocalhostToContainers {
		portBindings[nat.Port(portContainer)] = []nat.PortBinding{{HostIP: "127.0.0.1", HostPort: portLocalhost}}
	}

	remoteEnvVars := opts.RemoteEnvVars
	// merge remote env variables with .env file (if specified) with priority for .env file
	err = mergo.Merge(&remoteEnvVars, opts.LocalEnvVars, mergo.WithOverride)
	if err != nil {
		return errors.Wrap(err, "unable to merge .env file values with jetpack env values")
	}
	resp, err := cli.ContainerCreate(
		ctx,
		&container.Config{
			Image:        opts.BuildOut.Image.String(),
			ExposedPorts: nat.PortSet{"8080": struct{}{}, "8081": struct{}{}},
			Cmd:          sdkCmdSlice,
			Env:          goutil.Entries(remoteEnvVars),
		},

		&container.HostConfig{
			PortBindings: portBindings,
		},
		nil, // network config
		nil, // platform
		"",  // name is empty, makes Docker come up with random name
	)
	if err != nil {
		return errors.Wrap(err, "unable to create container")
	}

	err = cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
	if err != nil {
		return errors.Wrap(err, "unable to start container")
	}
	green.Fprintf(
		jetlog.Logger(ctx), "%s container started\n", opts.BuildOut.Image.Name())

	portMessages := []string{"Port Forwarding:"}
	for portLocalhost, portContainer := range portLocalhostToContainers {
		msg := fmt.Sprintf("localhost:%s -> %s port on container", portLocalhost, portContainer)
		portMessages = append(portMessages, msg)
	}
	green.Fprintf(jetlog.Logger(ctx), strings.Join(portMessages, "\n\t")+"\n")

	out, err := cli.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
	})
	green.Fprintln(jetlog.Logger(ctx), ("Listening to container logs..."))

	if err != nil {
		return errors.Wrap(err, "unable to get container logs")
	}

	defer func() {
		closeErr := out.Close()
		if err == nil {
			err = closeErr
		}
	}()

	_, err = stdcopy.StdCopy(os.Stdout, os.Stderr, out)

	if errors.Is(err, context.Canceled) {
		// Use new context since the original one is canceled
		return stopAndRemoveContainer(
			context.Background(),
			jetlog.Logger(ctx),
			cli,
			resp.ID,
		)
	} else if err != nil {
		return errors.Wrap(err, "failed to copy pipe to stdout and strerr")
	}

	statusCh, errCh := cli.ContainerWait(
		ctx,
		resp.ID,
		container.WaitConditionNotRunning,
	)
	select {
	case err := <-errCh:
		return errors.Wrap(err, "failed to wait for container status change")
	case <-statusCh:
		yellow.Fprintln(jetlog.Logger(ctx), "Container is no longer running. Skipping cleanup.")
	}

	return nil
}

func stopAndRemoveContainer(
	ctx context.Context,
	logger io.Writer,
	cli *client.Client,
	respID string,
) error {
	yellow.Fprintln(logger, "Attempting to stop container...")
	err := cli.ContainerStop(ctx, respID, nil)
	if err != nil {
		return errors.Wrap(err, "failed to stop container")
	}
	green.Fprintln(logger, "Container stopped successfully")

	yellow.Fprintln(logger, "Attempting to remove container...")
	err = cli.ContainerRemove(ctx, respID, types.ContainerRemoveOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to remove container")
	}
	green.Fprintln(logger, "Container removed successfully")
	return nil
}
