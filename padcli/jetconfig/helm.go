package jetconfig

// Public HelmChart interface
type HelmChart interface {
	Service
	GetRepo() string
	GetChart() string
}

// instantiates a new Job service for initcmd
func (c *Config) AddNewJobHelmChart(
	name string,
	command []string,
	schedule string,
) HelmChart {
	newHelmChart := &helmChart{
		service: service{
			parent: c,
			name:   name,
			Type:   HelmChartType,
		},
	}
	c.Services = append(c.Services, newHelmChart)
	return newHelmChart
}

// Private helmChart struct
type helmChart struct {
	service `yaml:",inline,omitempty"`
	Repo    string `yaml:"repo,omitempty"`
	Chart   string `yaml:"chart,omitempty"`
}

func (h *helmChart) GetRepo() string {
	return h.Repo
}

func (h *helmChart) GetChart() string {
	return h.Chart
}

var _ HelmChart = (*helmChart)(nil)

func (c *Config) HelmCharts() []HelmChart {
	result := []HelmChart{}
	for _, svc := range c.Services {
		if hc, ok := svc.(*helmChart); ok {
			result = append(result, hc)
		}
	}
	return result
}
