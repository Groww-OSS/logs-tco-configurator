package promtail

import (
	"gopkg.in/yaml.v2"
)

// PromtailConfig represents the promtail configuration for the application.
type PromtailConfig struct {
	Server        interface{}    `yaml:"server"`
	Client        interface{}    `yaml:"client"`
	Positions     interface{}    `yaml:"positions"`
	ScrapeConfigs []ScrapeConfig `yaml:"scrape_configs"`
}

type ScrapeConfig struct {
	JobName             string          `yaml:"job_name"`
	PipelineStages      []PipelineStage `yaml:"pipeline_stages"`
	KubernetesSDConfigs []yaml.MapSlice `yaml:"kubernetes_sd_configs"`
	RelabelConfigs      []yaml.MapSlice `yaml:"relabel_configs"`
}

type PipelineStage map[string]interface{}

type DropStage struct {
	Source            string `yaml:"source"`
	DropCounterReason string `yaml:"drop_counter_reason"`
	Value             string `yaml:"value"`
}

// MatchStage represents a sampling stage
type MatchStage struct {
	PipelineName string                `yaml:"pipeline_name"`
	Selector     string                `yaml:"selector"`
	Stages       []map[string]Sampling `yaml:"stages"`
}

type Sampling struct {
	Rate float64 `yaml:"rate"`
}

type workload string

type samplingPercentage float64
