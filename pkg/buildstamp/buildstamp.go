package buildstamp

import (
	"fmt"
	"io"
	"runtime"
	"strings"
)

// ldflags will provide these values. See devtools/scripts/build/ldflags.sh for
// details on how they are computed.
var (
	// User is the username of the account that built the binary.
	User string

	// Host is the unqualified hostname of the machine used to build the binary.
	Host string

	// BuildTimestamp is the timestamp at which the binary was built in ISO 8601
	// format.
	BuildTimestamp string

	// Branch is the name of the git branch used to build the binary.
	Branch string

	// Commit is the git commit hash of the revision used to build the binary.
	Commit string

	// CommitTimestamp is the timestamp of the commit used to build the binary in
	// ISO 8601 format.
	CommitTimestamp string

	// ReleaseTag is the tag of the revision used to build the binary as provided
	// by `git describe`. In general, it's something like: "a968903-dirty"
	ReleaseTag string

	// VersionTagTime is the time that this version of Jetpack (CLI binary, runtime and SDK)
	// was built, and tagged.
	VersionTagTime string

	// VersionNumber is the version number in semver format MAJOR.MINOR.PATCH
	VersionNumber string

	// PrereleaseTag tells us what edition of Jetpack's Pre-releases this is. Usually, "dev".
	PrereleaseTag string

	// CicdBuildRelease is set when the CLI binary was built through CICD
	CicdBuildRelease string

	// StableDockerTag is used for docker images and helm charts
	StableDockerTag string
)

type BuildStamper interface {
	Version() string
	IsCicdReleasedBinary() bool
	IsDevBinary() bool
}

type buildStamp struct{}

func Get() *buildStamp {
	return &buildStamp{}
}

// Version returns a short version string of the form: 0.1.0-dev20211005+379c1d11-dirty
// This is the subset of the semver formatting that is shared by python (for SDK)
// and runtime
func (b *buildStamp) Version() string {
	if strings.TrimSpace(PrereleaseTag) == "" {
		return VersionNumber
	}
	date := strings.ReplaceAll(VersionTagTime, ".", "")
	return strings.TrimSpace(fmt.Sprintf(
		"%s-%s%s+%s",
		VersionNumber,
		PrereleaseTag,
		date,
		ReleaseTag,
	))
}

// PrintVerboseVersion prints a verbose listing of the version variables
// to the io.Writer argument
func PrintVerboseVersion(w io.Writer) {
	fmt.Fprint(w, "\n")
	fmt.Fprintf(w, "Version Number: %v\n", VersionNumber)
	fmt.Fprintf(w, "Prerelease Tag: %v\n", PrereleaseTag)
	fmt.Fprintf(w, "Release:        %v\n", ReleaseTag)
	fmt.Fprintf(w, "Commit:         %v\n", Commit)
	fmt.Fprintf(w, "Branch:         %v\n", Branch)
	fmt.Fprintf(w, "Commit Date:    %v\n", CommitTimestamp)
	fmt.Fprint(w, "\n")
	fmt.Fprintf(w, "Build Date:  %v\n", BuildTimestamp)
	fmt.Fprintf(w, "Built by:    %v@%v\n", User, Host)
	fmt.Fprintf(w, "Runtime:     %v\n", runtime.Version())
	fmt.Fprintf(w, "CI/CD:       %v\n", CicdBuildRelease)
}

func (b *buildStamp) IsCicdReleasedBinary() bool {
	return CicdBuildRelease == "prod"
}

func (b *buildStamp) IsDevBinary() bool {
	// If this is missing, just assume dev.
	return CicdBuildRelease == "" || CicdBuildRelease == "dev"
}
