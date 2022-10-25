package semver

// semver package wraps golang's native "golang.org/x/mod/semver" package
//
// We wrap that for convenience and extra safety:
// 1. convenience: the external package requires version strings to be prefixed by "v"
//    i.e. "1.2.3" should be "v1.2.3"
// 2. safety: the Compare() function quietly accepts invalid values by
//    "An invalid semantic version string is considered less than a valid one.
//    All invalid semantic version strings compare equal to each other."
//    We should error for invalid values.
