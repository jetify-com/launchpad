package jetconfig

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"go.jetpack.io/launchpad/goutil/errorutil"
	"go.jetpack.io/launchpad/padcli/terminal"
	"go.jetpack.io/launchpad/pkg/jetlog"
	"go.jetpack.io/launchpad/proto/api"
	"gopkg.in/yaml.v3"
)

const defaultFileName = "launchpad.yaml"

type EnvironmentFields struct {
	// Default flags
	Flags FlagSet `yaml:"flags,omitempty"`
}

// TODO, make this struct unexported
type Config struct {
	ConfigVersion string `yaml:"configVersion,omitempty"`

	// typeid generated for each project for storing env vars without having to rely on
	// project name which may change over time
	ProjectID string `yaml:"projectId,omitempty"`

	Name string `yaml:"name,omitempty"`

	// The cluster to deploy to. Should be the unique name of a Jetpack-managed cluster,
	// or the name of a context in the user's kubeconfig.
	Cluster string `yaml:"cluster,omitempty"` // --cluster

	ImageRepository string `yaml:"imageRepository,omitempty"`

	Environment map[string]EnvironmentFields `yaml:"environment,omitempty"`

	Services services `yaml:"services,omitempty"`

	// The file path to this jetconfig
	Path string `yaml:"-"`

	// part of app state but not saved to yaml
	selectedEnvironment api.Environment
}

// isPathFormatAConfigFile returns true if the path looks like a config file even
// if the file doesn't exist
func isPathFormatAConfigFile(path string) bool {
	fi, err := os.Stat(path)
	if err == nil && !fi.IsDir() {
		return true
	}
	// Handle new configs:
	// There's probably better more general ways to do this, but this if ok for
	// now. Assume that if base contains a period it's a file.
	return strings.ContainsRune(filepath.Base(path), '.')
}

func configPath(path string) string {
	if isPathFormatAConfigFile(path) {
		return path
	}
	// if the old jetconfig.yaml still exists, use it instead of launchpad.yaml.
	configFilePath := filepath.Join(path, defaultFileName)
	legacyConfigFilePath := filepath.Join(path, "jetconfig.yaml")
	_, err := os.Stat(legacyConfigFilePath)
	if err == nil {
		return legacyConfigFilePath
	}
	return configFilePath
}

func ConfigName(path string) string {
	return filepath.Base(configPath(path))
}

func ConfigDir(path string) string {
	if isPathFormatAConfigFile(path) {
		return filepath.Dir(path)
	}
	return path
}

func (cfg *Config) SaveConfig(path string) (string, error) {
	marshalledYaml, err := cfg.marshalYaml()
	if err != nil {
		return "", errors.WithStack(err)
	}
	filePath := configPath(path)
	if err = os.WriteFile(filePath, marshalledYaml, 0666); err != nil {
		return "", errors.Wrapf(err, "failed to yaml marshal jetconfig: %v", cfg)
	}
	return filePath, nil
}

// RequireFromFileSystem will:
// - read launchpad.yaml at `path` in the file system
// - populates the Config struct from the file's contents, via yaml unmarshalling
// - upgrade the jetconfig with newer fields
// - if config doesn't exist, it returns an error.
func RequireFromFileSystem(
	ctx context.Context,
	path string,
	env api.Environment,
) (*Config, error) {
	filePath := configPath(path)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, ErrConfigNotFound
	}
	yamlContents, err := os.ReadFile(filePath)
	if err != nil {
		return nil, errors.Wrapf(
			err,
			"failed to read jetconfig file at %s",
			filePath,
		)
	}

	cfg := &Config{Path: filePath, selectedEnvironment: env}
	err = cfg.loadConfigFromYamlContents(yamlContents)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if err = cfg.upgrade(ctx, filePath); err != nil {
		return nil, errors.WithStack(err)
	}

	return cfg, cfg.validate()
}

func (c *Config) GetProjectID() string {
	return c.ProjectID
}

func (c *Config) GetProjectSlug() string {
	slug, err := GetProjectSlug(c.ProjectID)
	if err != nil {
		// fallback to using the whole project ID
		return c.ProjectID
	}
	return slug
}

func (c *Config) Cronjobs() []Cron {
	result := []Cron{}
	for _, svc := range c.Services {
		if c, ok := svc.(*cron); ok {
			result = append(result, c)
		}
	}
	return result
}

// For now, at most one web service is expected
// If no web service is specified, then returns empty-string for name, and nil for *Web
//
// NOTE: if we add a service.Name field, then this API improves
// TODO DEV-966 enable multiple web-services. Change function's name and type.
func (c *Config) WebService() (Web, error) {
	var websvc Web
	for _, svc := range c.Services {
		if w, ok := svc.(*web); ok {
			if websvc != nil {
				return nil, errorutil.NewUserError(
					"The jetconfig has multiple web services but " +
						"jetpack currently only supports a single web service.",
				)
			}

			websvc = w
		}
	}

	return websvc, nil
}

func (c *Config) Builders() map[string]Builder {
	result := map[string]Builder{}
	for _, svc := range c.Services {
		if b, ok := svc.(Builder); ok {
			result[svc.GetName()] = b
		}
	}
	return result
}

func GetServiceTypes() []string {
	return []string{
		CronType,
		WebType,
	}
}

// pulled out for testing
func (cfg *Config) marshalYaml() ([]byte, error) {
	var marshalledYaml bytes.Buffer
	// setting yaml indent to 2
	yamlEncoder := yaml.NewEncoder(&marshalledYaml)
	yamlEncoder.SetIndent(2)
	// encoding config into yaml format
	err := yamlEncoder.Encode(&cfg)

	return marshalledYaml.Bytes(), errors.Wrapf(err, "failed to yaml marshal jetconfig: %v", cfg)
}

// loadConfigFromYamlContents populates the `cfg Config` from the `yamlContents`.
//
// pulled out into its own function so we can write a test for
// the custom marshalling that `services` do.
func (cfg *Config) loadConfigFromYamlContents(yamlContents []byte) error {
	// Request for Comment:
	// Ideally, we would do strict so that any fields with a typo are caught.
	// Not doing Strict because if the jetconfigStruct has already been loaded and some code calls
	// this jetconfigStruct.LoadConfig again, then this will fail because the fields or keys will be duplicate.
	if err := yaml.Unmarshal(yamlContents, cfg); err != nil {
		return errors.Wrap(
			err,
			"failed to read jetconfig. yaml file due to mismatch of fields with the jetconfig struct",
		)
	}
	return nil
}

func (cfg *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type rawConfig Config // alias to prevent recursion
	if err := unmarshal((*rawConfig)(cfg)); err != nil {
		return err
	}

	for _, s := range cfg.Services {
		s.setParent(cfg)
	}

	return nil
}

func (cfg *Config) HasDeployment() (bool, error) {
	// We can evolve this to other types of services (like internal)
	w, err := cfg.WebService()
	return w != nil, errors.WithStack(err)
}

func (cfg *Config) GetProjectName() string {
	return cfg.Name
}

func (c *Config) GetProjectNameWithSlug() string {
	return fmt.Sprintf("%s-%s", c.GetProjectName(), c.GetProjectSlug())
}

func (cfg *Config) GetInstanceName() string {
	// For now we assume one web service, so the instance name
	// will simply be <projectName>-<serviceName>.
	w, err := cfg.WebService()
	if err != nil || w == nil {
		// No webservice found. Fallback to project name.
		return cfg.GetProjectName()
	}

	projectName := cfg.GetProjectName()
	serviceName := w.GetName()
	if strings.Contains(serviceName, projectName) {
		return serviceName
	}

	return fmt.Sprintf("%s-%s", projectName, serviceName)
}

// upgrade will edit the schema to match the latest version of the schema,
// and write the newer Config to the launchpad.yaml file.
// upgrade is called after `cfg Config` has been populated from the launchpad.yaml file.
//
// It focuses on upgrading from the latest-minus-one version to the latest version. It doesn't
// strive be compatible for much older versions: users are expected to auto-upgrade
// because they regularly deploy jetpack projects, or reach out for support if they
// need to manually update a very old project. This reduces our engineering burden
// of maintaining backwards compatibility for an alpha product, which rapidly
// evolves.
//
// NOTE: we'll need to periodically update this function to reflect changes to jetconfig
// and also purge upgrades for fields once they have been present for a long time.
func (cfg *Config) upgrade(ctx context.Context, filePath string) error {
	configFileName := ConfigName(filePath)
	if isUnsupported, err := isVersionLessThanMinimumSupported(cfg.ConfigVersion); err != nil {
		return errors.WithStack(err)
	} else if isUnsupported {
		return errorutil.NewUserErrorf(
			fmt.Sprintf("The configVersion in %s is too old to auto-upgrade. "+
				"Please reach out for support, or consult docs at https://www.jetpack.io/docs/reference/%s-reference/", configFileName, configFileName))
	}

	if needsUpgrade, err := doesVersionNeedUpgrade(cfg.ConfigVersion); err != nil {
		return errors.WithStack(err)
	} else if !needsUpgrade {
		// Nothing to upgrade
		return nil
	}

	// stop the upgrade if we are in a non-interactive terminal.
	if !terminal.IsInteractive() {
		return errorutil.NewUserErrorf(
			fmt.Sprintf("The configVersion in %s needs an upgrade. Please run `jetpack config upgrade`.", configFileName))
	}

	cfg.ConfigVersion = Versions.Prod()

	// Other upgraded fields go here:

	jetlog.Logger(ctx).WarningPrintf(
		"Upgraded your %s to the latest version %s. "+
			"Please commit this change to your repository.",
		configFileName,
		Versions.Prod(),
	)
	_, err := cfg.SaveConfig(filePath)
	return errors.Wrap(err, fmt.Sprintf("failed to auto-upgrade %s", configFileName))
}

func (c *Config) HasDefaultFileName() bool {
	return strings.HasSuffix(c.Path, defaultFileName)
}

func (c *Config) GetCluster() string {
	if c == nil {
		return ""
	}
	return c.Cluster
}
