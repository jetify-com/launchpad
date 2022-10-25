package launchpad

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"go.jetpack.io/launchpad/pkg/jetlog"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	// https://github.com/kubernetes/client-go/issues/242
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

// Move out of launchpad into some sort of kubernetes package
func RESTConfigFromDefaults(currentCtx string) (*rest.Config, error) {
	// Default Path: ~/.kube/config
	path := filepath.Join(homedir.HomeDir(), ".kube", "config")

	// Check that files exists and we can access it. If we can't, set to empty string.
	if _, err := os.Stat(path); err != nil {
		path = ""
	}

	// This method will try the kubeconfig in the given path
	clientCfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: path},
		&clientcmd.ConfigOverrides{
			CurrentContext: currentCtx,
		},
	).ClientConfig()
	return clientCfg, errors.WithStack(err)
}

func waitForPodNameForChart(
	ctx context.Context,
	ns string,
	config *rest.Config,
	labelSelector string,
) (string, error) {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", errors.Wrap(err, "Error creating k8s clientset")
	}

	watch, err := clientset.CoreV1().Pods(ns).Watch(ctx, v1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return "", errors.Wrap(err, "Error watching pods")
	}

	// Wait up to 10 seconds for pod to be live
	timeout := time.AfterFunc(10*time.Second, func() {
		jetlog.Logger(ctx).Println("Timeout waiting for pod to start")
		watch.Stop()
	})

	defer func() {
		watch.Stop()
		timeout.Stop()
	}()

	for event := range watch.ResultChan() {
		pod, ok := event.Object.(*corev1.Pod)
		if !ok {
			break // This is almost certainly a timeout error
		}
		// Ignore jobs
		if _, ok := pod.Labels["job-name"]; ok {
			continue
		}

		if pod.Status.Phase == corev1.PodRunning {
			return pod.Name, nil
		}
	}

	return "", errWaitForPodTimeout
}

func getSecretData(
	ctx context.Context,
	kubeCtx string,
	ns string,
	name string,
) (map[string][]byte, error) {
	config, err := RESTConfigFromDefaults(kubeCtx)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	secret, err := clientset.CoreV1().Secrets(ns).Get(ctx, name, v1.GetOptions{})
	if k8sErrors.IsNotFound(err) {
		return map[string][]byte{}, nil
	} else if err != nil {
		return nil, errors.WithStack(err)
	}

	return secret.Data, nil
}
