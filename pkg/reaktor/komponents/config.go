package komponents

import "fmt"

const secretsMountPath = "/var/run/secrets/jetpack.io"
const configMapMountPath = "/var/run/config/jetpack.io"

type EnvConfig interface {
	ToEnvFrom() []map[string]any
	ToVolumes() []map[string]any
	ToVolumeMounts() []map[string]any
}

type EmptyEnvConfig struct{}

func (*EmptyEnvConfig) ToEnvFrom() []map[string]any      { return nil }
func (*EmptyEnvConfig) ToVolumes() []map[string]any      { return nil }
func (*EmptyEnvConfig) ToVolumeMounts() []map[string]any { return nil }

// EmptyEnvConfig implements interface EnvConfig (compile-time check)
var _ EnvConfig = (*EmptyEnvConfig)(nil)

// ConfigRef implements EnvConfig and can provide configMap and
// Secrets based configuration based on refs
type ConfigRef struct {
	ConfigMapRef string
	SecretsRef   string
}

// ConfigRef implements interface EnvConfig (compile-time check)
var _ EnvConfig = (*ConfigRef)(nil)

func (c *ConfigRef) ToEnvFrom() []map[string]any {
	res := []map[string]any{}
	if c == nil {
		return res
	}

	if c.ConfigMapRef != "" {
		res = append(res, map[string]any{
			"configMapRef": map[string]any{
				"name": c.ConfigMapRef,
			},
		})
	}

	if c.SecretsRef != "" {
		res = append(res, map[string]any{
			"secretRef": map[string]any{
				"name": c.SecretsRef,
			},
		})
	}

	return res
}

func (c *ConfigRef) ToVolumes() []map[string]any {
	res := []map[string]any{}
	if c == nil {
		return res
	}

	if c.ConfigMapRef != "" {
		res = append(res, map[string]any{
			"name": c.configMapMountName(),
			"configMap": map[string]any{
				"name": c.ConfigMapRef,
			},
		})
	}

	if c.SecretsRef != "" {
		res = append(res, map[string]any{
			"name": c.secretMountName(),
			"secret": map[string]any{
				"secretName": c.SecretsRef,
			},
		})
	}

	return res
}

func (c *ConfigRef) ToVolumeMounts() []map[string]any {
	res := []map[string]any{}
	if c == nil {
		return res
	}

	if c.ConfigMapRef != "" {
		res = append(res, map[string]any{
			"name":      c.configMapMountName(),
			"mountPath": configMapMountPath,
			"readOnly":  true,
		})
	}

	if c.SecretsRef != "" {
		res = append(res, map[string]any{
			"name":      c.secretMountName(),
			"mountPath": secretsMountPath,
			"readOnly":  true,
		})
	}

	return res
}

func (c *ConfigRef) secretMountName() string {
	return fmt.Sprintf("secret-mount-%s", c.SecretsRef)
}

func (c *ConfigRef) configMapMountName() string {
	return fmt.Sprintf("config-map-mount-%s", c.SecretsRef)
}
