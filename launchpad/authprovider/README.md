This package is mostly copy pasted from buildkit main branch
https://github.com/moby/buildkit/tree/master/session/auth/authprovider
We need it for 2 reasons:

- Pass a custom config (instead of loading the default one)
- AuthProvider takes a struct instead of an interface which makes it challenging
  to customize even if we pulled the main branch as a dependency.

Changes:

- NewDockerAuthProvider takes interface instead of struct
- Seed is hard coded to temp directory instead of config file path
- Everything in config.go is our code (not from docker)
- Added some error handling in tokenseed to satisfy GolangCI linter

From Docker:

- All logic in tokenseed
- Everything in authprovider.go except the Config interface and NewDockerAuthProvider changes mentioned above
