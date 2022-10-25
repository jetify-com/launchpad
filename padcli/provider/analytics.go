package provider

import (
	"context"
)

type Analytics interface {
	Track(ctx context.Context, event string, options map[string]any)
	Close() error
}

type defaultAnalytics struct{}

func DefaultAnalyticsProvider() Analytics {
	return &defaultAnalytics{}
}

func (a *defaultAnalytics) Track(ctx context.Context, event string, options map[string]any) {
}

func (a *defaultAnalytics) Close() error {
	return nil
}
