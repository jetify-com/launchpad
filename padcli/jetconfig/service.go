package jetconfig

import (
	"fmt"
	"path/filepath"
	"strings"
)

// TODO DEV-983 consolidate default helm value constants
const DefaultAppPodPort = 8080

// Types
const (
	CronType      = "cron"
	HelmChartType = "helm"
	JobType       = "job"
	WebType       = "web"

	// other possible types:
	// internal
	// app - what is app??
	// worker
	// helm
)

// Service represents the core of a jetconfig service
type Service interface {
	GetName() string
	GetUniqueName() string
	GetPath() string

	setName(name string)
	setParent(p *Config)
}

// Lets consider getting rid of this struct
type service struct {
	parent *Config
	name   string
	Type   string `yaml:"type"`
}

func (s *service) GetName() string {
	return s.name
}

func (s *service) GetPath() string {
	return filepath.Dir(s.parent.Path)
}

func (s *service) setParent(p *Config) {
	s.parent = p
}

func (s *service) setName(name string) {
	s.name = name
}

// Unique name for k8s resources. For now, the format is <projectName>-<serviceName>
// This does not guarantee uniqueness, but significantly reduce chances of collision.
func (t *service) GetUniqueName() string {
	projectName := t.parent.GetProjectName()
	serviceName := t.name
	if strings.Contains(serviceName, projectName) {
		return serviceName
	}

	return fmt.Sprintf("%s-%s", projectName, serviceName)
}

// Builder is an interface for services that may have BuildCommand fields.
// NOTE: not all builders will eventually have image. For example, helm-services
// may be builders but may not have images. We may refactor when introducing
// such builders.
type Builder interface {
	GetBuildCommand() string
	GetImage() string
	GetInstanceType() *InstanceType
	GetPath() string
	ShouldPublish() bool
}

type builder struct {
	BuildCommand string        `yaml:"buildCommand,omitempty,flow"`
	Image        string        `yaml:"image,omitempty"`
	InstanceType *InstanceType `yaml:"instance,omitempty"`
}

func (b *builder) GetBuildCommand() string {
	return InterpolateStamps(b.BuildCommand)
}

func (b *builder) GetImage() string {
	img := b.Image
	if b.ShouldPublish() {
		img = strings.Replace(b.Image, "local/", "", 1)
	}
	return InterpolateStamps(img)
}

func (b *builder) GetInstanceType() *InstanceType {
	return b.InstanceType
}

func (b *builder) ShouldPublish() bool {
	return strings.HasPrefix(b.Image, "local/")
}
