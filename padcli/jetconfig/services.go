package jetconfig

import (
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"gopkg.in/yaml.v3"
)

type services []Service

func (s *services) UnmarshalYAML(value *yaml.Node) error {
	result := services{}

	// each pair is a (name, serviceStruct) of type (string, map)
	for _, pair := range lo.Chunk(value.Content, 2) {
		if len(pair) != 2 || pair[0].Kind != yaml.ScalarNode || pair[1].Kind != yaml.MappingNode {
			return errors.WithStack(errors.New("invalid service definition"))
		}

		rawService := &struct{ Type string }{}
		if err := pair[1].Decode(rawService); err != nil {
			return errors.WithStack(err)
		}

		var svc Service
		switch rawService.Type {
		case CronType:
			svc = &cron{}
		case HelmChartType:
			svc = &helmChart{}
		case JobType:
			svc = &job{}
		case WebType:
			svc = &web{}
		default:
			return errors.Errorf("unknown service type: %s", rawService.Type)
		}

		if err := pair[1].Decode(svc); err != nil {
			return errors.WithStack(err)
		}

		svc.setName(pair[0].Value)
		result = append(result, svc)
	}
	*s = result

	return nil
}

func (svcs services) MarshalYAML() (any, error) {
	node := yaml.Node{
		Kind: yaml.MappingNode,
	}

	// We want to marshal the services in a map form, but we implement this custom marshalling
	// because the builtin map marshaller sorts the keys. We want to preserve the order of the keys.
	for _, svc := range svcs {
		nameNode := &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: svc.GetName(),
		}
		node.Content = append(node.Content, nameNode)

		valueNode := &yaml.Node{}
		err := valueNode.Encode(svc)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		node.Content = append(node.Content, valueNode)
	}

	return node, nil
}
