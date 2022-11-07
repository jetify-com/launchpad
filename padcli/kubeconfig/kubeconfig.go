package kubeconfig

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/tools/clientcmd/api/latest"
	"k8s.io/client-go/util/homedir"
	"sigs.k8s.io/yaml"
)

var (
	errWrongNumberOfContexts = errors.New("Error, wrong number of contexts")
)

func MergeAndWrite(newConfig string) error {
	newConfigPath, err := writeTmpFile(newConfig)
	if err != nil {
		return errors.Wrap(err, "Error writing kubeconfig to tmp path")
	}

	// Default Path: ~/.kube/config
	path := filepath.Join(homedir.HomeDir(), ".kube", "config")

	rules := clientcmd.ClientConfigLoadingRules{
		Precedence: []string{
			path,
			newConfigPath,
		},
	}

	mergedConfig, err := rules.Load()
	if err != nil {
		return errors.Wrap(err, "Error loading kubeconfigs")
	}

	var newConfigObj struct {
		Contexts []struct {
			Name string
		}
	}
	err = yaml.Unmarshal([]byte(newConfig), &newConfigObj)
	if err != nil {
		return errors.Wrap(err, "Error parsing new kubeconfig")
	}
	if len(newConfigObj.Contexts) != 1 {
		return errors.Wrapf(
			errWrongNumberOfContexts,
			"Found %d contexts, expected 1",
			len(newConfigObj.Contexts),
		)
	}

	encodedConfig, err := runtime.Encode(latest.Codec, mergedConfig)
	if err != nil {
		return errors.Wrap(err, "Error encoding kubeconfig")
	}

	// TODO validate that there are no conflicts

	err = writeFile(path, encodedConfig)
	return errors.Wrap(err, "Error writing new kubeconfig")
}

func MergeAllAndWrite(kubeconfigs []string) error {
	for _, kc := range kubeconfigs {
		err := MergeAndWrite(kc)
		if err != nil {
			return errors.Wrap(err, "Error merging kubeconfig")
		}
	}
	return nil
}

func writeTmpFile(newConfig string) (string, error) {
	file, err := os.CreateTemp("", "prefix")
	if err != nil {
		return "", errors.Wrap(err, "Error creating tmp file")
	}
	_, err = file.WriteString(newConfig)
	if err != nil {
		return "", errors.Wrap(err, "Error writing config to tmp file")
	}

	return file.Name(), nil
}

func writeFile(path string, data []byte) error {
	if index := strings.LastIndex(path, "/"); index != -1 {
		dir := path[:index+1]
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			if err := os.MkdirAll(dir, 0744); err != nil {
				return err
			}
		}
	}
	return os.WriteFile(path, data, 0664)
}

func getRawKubeConfig() (api.Config, error) {
	cfg := GetKubeConfig()
	rawCfg, err := cfg.RawConfig()
	return rawCfg, errors.Wrap(err, "failed to get client config")
}

func GetKubeConfig() clientcmd.ClientConfig {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	// if you want to change the loading rules (which files in which order), you can do so here

	configOverrides := &clientcmd.ConfigOverrides{}
	// if you want to change override values or bind them to flags, there are methods to help you

	cfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	return cfg
}

func HasContext(ctxName string) (bool, error) {
	rawCfg, err := getRawKubeConfig()
	if err != nil {
		return false, errors.WithStack(err)
	}
	_, ok := rawCfg.Contexts[ctxName]
	return ok, nil
}

func GetContext(ctxName string) (*api.Context, error) {
	rawCfg, err := getRawKubeConfig()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	value, ok := rawCfg.Contexts[ctxName]
	if !ok {
		return nil, errors.Errorf("No kube context named \"%s\" found", ctxName)
	}
	return value, nil
}

func GetCurrentContextName() (string, error) {
	rawCfg, err := getRawKubeConfig()
	if err != nil {
		return "", errors.WithStack(err)
	}
	return rawCfg.CurrentContext, nil
}

// GetContextNames returns a slice with the names of the contexts in the kubeconfig. The
// slice may be empty.
func GetContextNames() ([]string, error) {
	rawCfg, err := getRawKubeConfig()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	names := make([]string, len(rawCfg.Contexts))
	i := 0
	for k := range rawCfg.Contexts {
		names[i] = k
		i++
	}

	return names, nil
}

func GetAuthInfo(authInfoName string) (*api.AuthInfo, error) {
	rawCfg, err := getRawKubeConfig()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if info, ok := rawCfg.AuthInfos[authInfoName]; ok {
		return info, nil
	} else {
		return nil, errors.Errorf("no AuthInfo found with name %s", authInfoName)
	}
}

func GetServer(context string) (string, error) {
	rawCfg, err := getRawKubeConfig()
	if err != nil {
		return "", errors.WithStack(err)
	}

	currentKubeContext, ok := rawCfg.Contexts[context]
	if !ok {
		return "", errors.Wrap(err, "failed to get current kube context")
	}
	currentCluster, ok := rawCfg.Clusters[currentKubeContext.Cluster]
	if !ok {
		return "", errors.Wrap(err, "failed to get current cluster")
	}
	return currentCluster.Server, nil
}

func SetCurrentContext(ctx string) error {
	cfg, err := GetKubeConfig().RawConfig()
	if err != nil {
		return errors.WithStack(err)
	}
	cfg.CurrentContext = ctx
	return clientcmd.WriteToFile(cfg, filepath.Join(homedir.HomeDir(), ".kube", "config"))
}
