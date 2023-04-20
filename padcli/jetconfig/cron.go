package jetconfig

// Public Cron interface
type Cron interface {
	Builder
	Service
	GetSchedule() string
	GetConcurrencyPolicy() string
	GetCommand() []string
}

// instantiates a new Cron service for initcmd
func (c *Config) AddNewCronService(
	name string,
	command []string,
	schedule string,
) Cron {
	newCron := &cron{
		Command:  command,
		Schedule: schedule,
		builder: builder{
			Image: "busybox:latest",
		},
		service: service{
			parent: c,
			name:   name,
			Type:   CronType,
		},
	}
	c.Services = append(c.Services, newCron)
	return newCron
}

// Private cron struct
type cron struct {
	service          `yaml:",inline,omitempty"`
	builder          `yaml:",inline,omitempty"`
	Command          []string `yaml:"command,omitempty,flow"`
	Schedule         string   `yaml:"schedule,omitempty"`
	ConcurrentPolicy string   `yaml:"concurrencyPolicy,omitempty"`
}

var _ Cron = (*cron)(nil)

func (c *cron) GetSchedule() string {
	return c.Schedule
}

func (c *cron) GetConcurrencyPolicy() string {
	if c.ConcurrentPolicy == "" {
		return "Allow" // k8s default
	}
	return c.ConcurrentPolicy
}

func (c *cron) GetCommand() []string {
	return c.Command
}
