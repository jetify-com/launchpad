package docker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/docker/docker/api/types"
)

func Build(ctx context.Context, path string, opts types.ImageBuildOptions) error {
	cmd := command(ctx, "docker", "build", path)
	cmd.Env = append(os.Environ(), "DOCKER_BUILDKIT=1")
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
	fmt.Fprintln(os.Stderr, cmd.String())
	return cmd.Run()
}

func command(ctx context.Context, name string, arg ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, arg...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}
