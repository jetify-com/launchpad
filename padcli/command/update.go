package command

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.jetpack.io/launchpad/pkg/buildstamp"
	"go.jetpack.io/launchpad/pkg/jetlog"
	"golang.org/x/sys/unix"
)

const scriptURL = "https://get.jetpack.io/"
const latestNightlyVersionURL = "https://releases.jetpack.io/jetpack/nightly/version"

// This is a quick and dirty implementation of update that downloads and
// runs our existing install script.
// Note that it only works on Linux/BSD systems and it assumes that launchpad is
// installed in /usr/bin/local. If you call launchpad update from a binary that is
// not in that location, it will still update that location.
func updateCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "update",
		Short:  "update version",
		Args:   cobra.NoArgs,
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			buildstmp := buildstamp.Get()
			err := updateLaunchpad(cmd.Context(), cmd, args, buildstmp)
			return errors.Wrap(err, "failed to do update")
		},
	}
}

// updateLaunchpad will:
//  1. query S3 for the latest version of the latest binary, and compare it to
//     the current version.
//  2. If the versions are different, it will download the latest binary and then
//     use the latest binary to execute the current launchpad-command.
//
// The launchpad-command "update" is special-cased internally. In this case,
// it will ask the user for confirmation to update.
func updateLaunchpad(ctx context.Context, cmd *cobra.Command, args []string, buildstmp buildstamp.BuildStamper) error {
	if !buildstmp.IsCicdReleasedBinary() {
		jetlog.Logger(ctx).Println("You are in development mode. Not checking for updates.")
		return nil
	}

	currentVersion := buildstmp.Version()
	latestVersion, err := fetchLatestVersion()
	if err != nil {
		return errors.WithStack(err)
	}

	if currentVersion != latestVersion {
		jetlog.Logger(ctx).Println("A newer version of launchpad is available. Updating.")

		installScript, err := DownloadScript(scriptURL)
		if err != nil {
			return errors.Wrap(err, "Failed to add download the install.sh script")
		}

		installArgs := []string{"ignoredParam"}

		// We special-case the "update" command to ask for user-confirmation,
		// and don't call "--exec update" on the newly downloaded launchpad binary.
		if cmd.CalledAs() != "update" {
			// example: for command path `launchpad auth login`,
			// drop the `launchpad` and leave `auth login`
			installArgs = append(installArgs, "-y", "--exec")
			// add flags
			installArgs = append(installArgs, os.Args[1:]...)
		}

		err = unix.Exec(installScript, installArgs, os.Environ())
		if err != nil {
			return errors.WithStack(err)
		}
	} else if cmd.CalledAs() == "update" {
		jetlog.Logger(ctx).Println("Launchpad version is up-to-date.")
	}
	return nil
}

func fetchLatestVersion() (string, error) {
	resp, err := http.Get(latestNightlyVersionURL)
	if err != nil {
		return "", errors.WithStack(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	return strings.TrimSpace(string(body)), errors.WithStack(err)

}

func DownloadScript(url string) (string, error) {
	tmpDir, err := os.MkdirTemp("/tmp", "launchpad-installer")
	if err != nil {
		return "", errors.Wrapf(err, "Could not create temp directory")
	}

	scriptPath := filepath.Join(tmpDir, "install.sh")

	if _, err := os.Stat(scriptPath); err != nil {

		tmpFile, err := os.CreateTemp(tmpDir, "download")
		if err != nil {
			return "", errors.Wrapf(err, "Could not create temp file in %s", tmpDir)
		}
		defer func() {
			err := tmpFile.Close()
			if err == nil {
				os.Remove(tmpFile.Name())
			}
		}()

		resp, err := http.Get(url)
		if err != nil {
			return "", errors.Wrapf(err, "Download from %v failed", url)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			return "", errors.Wrapf(err, "Download from %v failed with error %v", url, resp.StatusCode)
		}

		_, err = io.Copy(tmpFile, resp.Body)
		if err != nil {
			return "", errors.Wrapf(err, "Could not copy data from %v to %v", url, tmpFile.Name())
		}

		err = os.Chmod(tmpFile.Name(), 0755)
		if err != nil {
			return "", errors.Wrapf(err, "Could not change permissions for file %v", tmpFile.Name())
		}

		err = os.Rename(tmpFile.Name(), scriptPath)
		if err != nil {
			return "", errors.Wrapf(err, "Could not move file %v to %v", tmpFile.Name(), scriptPath)
		}
	}

	return scriptPath, nil
}
