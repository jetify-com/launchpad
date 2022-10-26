package command

import (
	"context"

	"github.com/spf13/cobra"
	"go.jetpack.io/launchpad/padcli/command/mock"
)

// This simply runs the persistentPreRun function under root.go.
// We need to make sure that persistentPreRun never panics or else
// auto-update will be stuck in a forever loop.
func (t *Suite) TestPersistentPreRun() {
	req := t.Require()
	ctx := context.Background()

	dummyCmd := &cobra.Command{
		Use:   "dummy",
		Short: "authenticates the user with Launchpad",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	err := dummyCmd.ExecuteContext(ctx)
	req.NoError(err)

	err = persistentPreRunE(dummyCmd, []string{})
	req.NoError(err)
}

// Making sure the auto-update loop will stop if the version
// is indeed the newest version.
func (t *Suite) TestNoUpdate() {
	req := t.Require()
	ctx := context.Background()

	cmd := upCmd()
	buildstmp := &mock.MockBuildStamp{
		VersionFunc: func() string {
			v, err := fetchLatestVersion()
			req.NoError(err)
			return v
		},
	}

	err := updateLaunchpad(ctx, cmd, []string{}, buildstmp)
	req.NoError(err)
}
