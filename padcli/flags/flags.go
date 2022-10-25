package flags

import (
	"strings"

	"go.jetpack.io/launchpad/proto/api"
)

type RootCmdFlags struct {
	Debug            bool
	Environment      string
	SkipVersionCheck bool
}

func (f *RootCmdFlags) IsValidEnvironment() bool {
	return f.Env() != api.Environment_NONE
}

func (f *RootCmdFlags) Env() api.Environment {
	return api.Environment(api.Environment_value[strings.ToUpper(f.Environment)])
}
