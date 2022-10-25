package k8s

import (
	"context"
	"encoding/base64"

	"github.com/pkg/errors"
	"github.com/samber/lo"
	"go.jetpack.io/launchpad/pkg/reaktor"
	"go.jetpack.io/launchpad/pkg/reaktor/komponents"
	"go.jetpack.io/launchpad/pkg/reaktor/kubeconfig"
)

func AddSecretKey(
	ctx context.Context,
	kubeCtx string,
	secretName string,
	ns string,
	keyValue lo.Entry[string, string], // pass raw value, not base64 encoded
) error {
	klient, err := reaktor.WithClientBuilder(
		kubeconfig.NewClientBuilder(kubeconfig.WithFlags(&kubeconfig.Flags{
			Context: kubeCtx,
		})),
	)

	if err != nil {
		return errors.WithStack(err)
	}

	u, err := klient.Get(
		ctx,
		reaktor.SecretGVR(),
		secretName,
		ns,
	)
	if err != nil {
		return errors.WithStack(err)
	}

	s := komponents.SecretFromUnstructured(u)

	s.Data[keyValue.Key] = base64.StdEncoding.EncodeToString(
		[]byte(keyValue.Value),
	)
	_, err = klient.Apply(ctx, s)

	return errors.WithStack(err)
}
