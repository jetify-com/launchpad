package jetconfig

import (
	"net/url"
	"regexp"
	"strings"

	"go.jetpack.io/launchpad/proto/api"
)

type Web interface {
	Builder
	Service
	GetPort() int
	GetURL() (*url.URL, error)
}

// instantiates a new Web service for initcmd
func (c *Config) AddNewWebService(name string) Web {
	webSvc := &web{
		Port: DefaultAppPodPort,
		service: service{
			parent: c,
			name:   name,
			Type:   WebType,
		},
	}
	c.Services = append(c.Services, webSvc)
	return webSvc
}

type web struct {
	service `yaml:",inline,omitempty"`
	builder `yaml:",inline,omitempty"`
	// Port may be used by other services like "internal",
	// so we may move this to an interface
	Port int                       `yaml:"port,omitempty"`
	URL  envDependentField[string] `yaml:"url,omitempty"`
}

func (w *web) GetPort() int {
	if w == nil || w.Port == 0 {
		return DefaultAppPodPort
	}
	return w.Port
}

var scheme = regexp.MustCompile(`^https?://`)

func (w *web) GetURL() (*url.URL, error) {
	if w == nil {
		return nil, nil
	}
	u, ok := w.URL[w.parent.selectedEnvironment]
	if !ok && w.parent.selectedEnvironment == api.Environment_PROD {
		// only use non-env url if prod
		u = w.URL[api.Environment_NONE]
	}
	if strings.HasPrefix(u, "/") || scheme.MatchString(u) {
		return url.Parse(u)
	}
	return url.Parse("https://" + u)
}

var _ Web = (*web)(nil)
