package jetconfig

// Public Job interface
type Job interface {
	Builder
	Service
	GetCommand() []string
}

// instantiates a new Job service for initcmd
func (c *Config) AddNewJobService(
	name string,
	command []string,
	schedule string,
) Job {
	newJob := &job{
		Command: command,
		builder: builder{
			Image: "busybox:latest",
		},
		service: service{
			parent: c,
			name:   name,
			Type:   JobType,
		},
	}
	c.Services = append(c.Services, newJob)
	return newJob
}

// Private job struct
type job struct {
	service `yaml:",inline,omitempty"`
	builder `yaml:",inline,omitempty"`
	Command []string `yaml:"command,omitempty,flow"`
}

var _ Job = (*job)(nil)

func (c *job) GetCommand() []string {
	return c.Command
}

func (c *Config) Jobs() []Job {
	result := []Job{}
	for _, svc := range c.Services {
		if c, ok := svc.(*job); ok {
			result = append(result, c)
		}
	}
	return result
}
