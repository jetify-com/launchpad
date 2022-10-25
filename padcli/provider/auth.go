package provider

import (
	"context"
)

type User interface {
	Email() string
	Token() string
	OrgID() string
	ID() string
}

type Auth interface {
	Identify(ctx context.Context) (context.Context, error)
	User(ctx context.Context) (User, error)
}

type anonymous struct{}

func Anonymous() Auth {
	return &anonymous{}
}

func (*anonymous) Identify(ctx context.Context) (context.Context, error) {
	return ctx, nil
}

func (*anonymous) User(ctx context.Context) (User, error) {
	return nil, nil
}
