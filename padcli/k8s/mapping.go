package k8s

import (
	"context"
	"net"
	"strings"

	"github.com/pkg/errors"
	"go.jetpack.io/launchpad/goutil/errorutil"
	"go.jetpack.io/launchpad/pkg/reaktor"
	"go.jetpack.io/launchpad/pkg/reaktor/kubeconfig"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func MappingHostnameWithPath(
	ctx context.Context,
	name, ns, kubeCtx string,
) (string, error) {
	klient, err := reaktor.WithClientBuilder(
		kubeconfig.NewClientBuilder(kubeconfig.WithFlags(&kubeconfig.Flags{
			Context: kubeCtx,
		})),
	)
	if err != nil {
		return "", errors.WithStack(err)
	}

	m, err := klient.Get(
		ctx,
		reaktor.AmbassadorMappingGVR(),
		name,
		ns,
	)
	if err != nil {
		return "", errors.WithStack(err)
	}

	host, found, err := unstructured.NestedString(m.Object, "spec", "hostname")
	if err != nil {
		return "", errors.WithStack(err)
	} else if !found {
		return "", errorutil.NewUserError("host not found on ambassador mapping")
	}

	prefix, found, err := unstructured.NestedString(m.Object, "spec", "prefix")
	if err != nil {
		return "", errors.WithStack(err)
	} else if !found {
		return "", errorutil.NewUserError("prefix not found on ambassador mapping")
	}

	if !strings.ContainsRune(host, ':') {
		return host + prefix, nil
	}

	hostname, _, err := net.SplitHostPort(host)
	if err != nil {
		return "", errors.WithStack(err)
	}

	return hostname + prefix, nil
}
