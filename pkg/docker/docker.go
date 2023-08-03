package docker

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"go.jetpack.io/launchpad/goutil/errorutil"
)

type BuildOpts struct {
	BuildArgs  map[string]*string
	Dockerfile string
	Labels     map[string]string
	Platform   string
	Tags       []string
}

func Build(ctx context.Context, path string, opts BuildOpts) error {
	if err := ensureDocker(); err != nil {
		return err
	}

	// TODO implement remote-cache
	cmd := exec.CommandContext(ctx, "docker", "build", path)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	for _, tag := range opts.Tags {
		cmd.Args = append(cmd.Args, "-t", tag)
	}
	if opts.Platform != "" {
		cmd.Args = append(cmd.Args, "--platform", opts.Platform)
	}
	for k, v := range opts.Labels {
		cmd.Args = append(cmd.Args, "--label", k+"="+v)
	}
	if opts.Dockerfile != "" {
		cmd.Args = append(cmd.Args, "-f", filepath.Join(path, opts.Dockerfile))
	}
	for k, v := range opts.BuildArgs {
		cmd.Args = append(cmd.Args, "--build-arg", fmt.Sprintf("%s=%s", k, *v))
	}
	if os.Getenv("SSH_AUTH_SOCK") != "" {
		cmd.Args = append(cmd.Args, "--ssh", "default")
	}
	fmt.Fprintf(os.Stderr, "Running command: %s\n", cmd.String())
	return cmd.Run()
}

func ensureDocker() error {
	_, err := exec.LookPath("docker")
	if err != nil {
		return errorutil.NewUserError(
			"docker not found in PATH. Ensure Docker is installed and in your PATH.")
	}
	cmd := exec.Command("docker", "version", "--format", "{{.Server.Version}}")
	out, err := cmd.Output()
	if err != nil {
		return errorutil.NewUserError(
			"failed to get Docker daemon version. Ensure Docker daemon is running.")
	}
	version := string(bytes.TrimSpace(out))
	if version < "1.39" {
		return errOldDockerAPIVersion
	}
	fmt.Fprintf(
		os.Stderr,
		"Using Docker daemon version: %s\n",
		version,
	)
	return nil
}
