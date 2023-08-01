package docker

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/docker/docker/api/types"
	"go.jetpack.io/launchpad/pkg/cmdutil"
)

func Build(path string, opts types.ImageBuildOptions) error {
	cmd := cmdutil.CommandTTY("docker", "build", path)
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
