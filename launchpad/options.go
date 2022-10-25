package launchpad

import "time"

type HelmOptions struct {
	ChartLocation string
	InstanceName  string // display name for helm install
	ReleaseName   string // app identifier for helm install
	Timeout       time.Duration
	Values        map[string]any
}
