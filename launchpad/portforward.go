package launchpad

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/pkg/errors"
	"go.jetpack.io/launchpad/goutil"
	"go.jetpack.io/launchpad/pkg/jetlog"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

const (
	// TODO DEV-983 consolidate default helm value constants
	defaultPodPort = 8080

	// Please keep these in sync with the corresponding constants used in the
	// jetpack-internal clusterless package's runtime server.
	appPort     = 8080
	runtimePort = 8090
)

// We could make this a pseudo-string-enum via:
// type PortFwdTarget string
const (
	PortFwdTargetApp     = "app"
	PortFwdTargetRuntime = "runtime"
)

type PortForwardOptions struct {
	AppOrRuntime string // 'app' or 'runtime'
	DeployOut    *DeployOutput
	KubeCtx      string
	PodPort      int
	Target       string
}

type portForwardAPodRequest struct {
	// RestConfig is the kubernetes config
	RestConfig *rest.Config
	// Pod is the selected pod for this port forwarding
	Pod v1.Pod
	// LocalPort is the local port that will be selected to expose the PodPort
	LocalPort int
	// PodPort is the target port for the pod
	PodPort int
	// Steams configures where to write or read input from
	Streams genericclioptions.IOStreams
	// StopCh is the channel used to manage the port forward lifecycle
	StopCh <-chan struct{}
	// ReadyCh communicates when the tunnel is ready to receive traffic
	ReadyCh chan struct{}
}

func (pad *Pad) portForward(ctx context.Context, opts *PortForwardOptions) error {
	switch opts.Target {
	case PortFwdTargetApp:
		return errors.WithStack(portForwardForApp(ctx, opts))
	case PortFwdTargetRuntime:
		return errors.WithStack(portForwardForRuntime(ctx, opts))
	default:
		return errors.Errorf("unrecognized Port Forward Target: %s", opts.Target)
	}
}

func portForwardForApp(ctx context.Context, opts *PortForwardOptions) error {
	do := opts.DeployOut
	if err := validateRelease(do, AppChartName); err != nil {
		return errors.Wrap(err, "unable to tail logs and port forward after deployment")
	}

	jetlog.Logger(ctx).Println("Attempting to port forward app...")
	podPort := goutil.Coalesce(opts.PodPort, defaultPodPort)
	labelSelector := getPodLabelForApp(do.InstanceName, do.Releases[AppChartName].Version)
	return portForward(ctx, opts.KubeCtx, do.Namespace, appPort, podPort, labelSelector)
}

func portForwardForRuntime(ctx context.Context, opts *PortForwardOptions) error {
	// TODO, check runtime is installed. It's not as simple as using validateRelease
	// because runtime might already be installed, but not part of current release

	// Currently, if runtime is not installed, the waitForPodNameForChart will
	// timeout. This is OK, since no one uses jetpack without runtime, but if
	// that use case becomes more common this needs to be fixed.
	jetlog.Logger(ctx).Println("Attempting to port forward runtime...")

	podPort := goutil.Coalesce(opts.PodPort, defaultPodPort)
	labelSelector := getPodLabelForRuntime()
	return portForward(ctx, opts.KubeCtx, opts.DeployOut.Namespace, runtimePort, podPort, labelSelector)
}

// Inspired by https://github.com/gianarb/kube-port-forward/blob/master/main.go
func portForward(
	ctx context.Context,
	kubeCtx string,
	ns string,
	localPort int,
	podPort int,
	labelSelector string,
) (err error) {
	config, err := RESTConfigFromDefaults(kubeCtx)
	if err != nil {
		return errors.Wrap(err, "Error build k8s config")
	}

	podName, err := waitForPodNameForChart(ctx, ns, config, labelSelector)
	if err != nil {
		return errors.Wrap(err, "Error getting pod name")
	}

	// stopCh control the port forwarding lifecycle. When it gets closed the
	// port forward will terminate
	stopCh := make(chan struct{}, 1)
	// readyCh communicate when the port forward is ready to get traffic
	readyCh := make(chan struct{})
	// stream is used to tell the port forwarder where to place its output or
	// where to expect input if needed. For the port forwarding we just need
	// the output eventually
	stream := genericclioptions.IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}

	// The stopCh gets closed to gracefully handle its termination.
	go func() {
		<-ctx.Done()
		jetlog.Logger(ctx).Println("Stopping port-forward.")
		close(stopCh)
	}()

	if err != nil {
		return errors.Wrap(err, "failed to get pod port")
	}

	go func() {
		// PortForward the pod specified from its port 80 to the local port
		// 8080
		portForwardError := portForwardAPod(portForwardAPodRequest{
			RestConfig: config,
			Pod: v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      podName,
					Namespace: ns,
				},
			},
			LocalPort: localPort,
			PodPort:   podPort,
			Streams:   stream,
			StopCh:    stopCh,
			ReadyCh:   readyCh,
		})
		if err == nil {
			err = errors.Wrap(portForwardError, "Error port forwarding")
		}
	}()

	<-readyCh
	return nil
}

func portForwardAPod(req portForwardAPodRequest) error {
	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward",
		req.Pod.Namespace, req.Pod.Name)
	hostIP := strings.TrimLeft(req.RestConfig.Host, "htps:/")

	transport, upgrader, err := spdy.RoundTripperFor(req.RestConfig)
	if err != nil {
		return errors.Wrap(err, "Error creating round tripper")
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, &url.URL{Scheme: "https", Path: path, Host: hostIP})
	fw, err := portforward.New(
		dialer,
		[]string{fmt.Sprintf("%d:%d", req.LocalPort, req.PodPort)},
		req.StopCh,
		req.ReadyCh,
		nil, // output writer
		req.Streams.ErrOut,
	)
	if err != nil {
		return errors.Wrap(err, "Error creating port forwarder")
	}

	fmt.Fprintf(req.Streams.Out,
		"The service will be accessible at http://localhost:%d\n"+
			"\t - this port forwards to port %d on the pod\n",
		req.LocalPort,
		req.PodPort,
	)
	err = fw.ForwardPorts()
	return errors.WithStack(err)
}
