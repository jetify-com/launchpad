// Package kubeconfig makes it easy to create K8s clients from kubeconfig files.
//
// There are two ways to pass a kubeconfig file. Either the contexts of the
// kubeconfig file can be passed directly:
//
//	```
//	kubeconfig.FromData(kubeconfigContents).ToDynamicClient()
//	```
//
// Or you can read the kubeconfig file from a path, optionally specifying
// a kube-context and namespace:
//
//	```
//	args := kubeconfig.Args {
//	  KubeconfigPath: "path/to/kubeconfig"
//	  Context:        "my-default-cluster"
//	  Namespace:      "restrict-to-this-namespace"
//	}
//	kubeconfig.FromArgs(args).ToDynamicClient()
//	```
//
// The default constructor follows similar rules to kubectl. It looks for a kubeconfig
// file in your the path specified by $KUBECONFIG or your home directory "${HOME}/.kube/config"
// and defaults to the currently selected kube-context.
// If no kubeconfig exists in those paths, it will attempt to load credentials
// from the current cluster (if the binary is running inside of a k8s cluster)
//
//	```
//	kubeconfig.FromDefaults().ToDynamicClient()
//	```
package kubeconfig
