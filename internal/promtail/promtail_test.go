package promtail

import (
	"reflect"
	"testing"
)

const sampleConfig = `
server:
  log_level: info
  http_listen_port: 3101
client:
  url: http://0.0.0.0:80/loki/api/v1/push
  tenant_id: x-org
  external_labels:
    cluster: cluster-002
positions:
  filename: /run/promtail/positions.yaml
scrape_configs:
  - job_name: kubernetes-pods
    pipeline_stages:
      - docker: null
      - cri: null
      - multiline:
          firstline: \d{4}-\d{2}-\d{2} \d{1,2}:\d{2}:\d{2}
          max_wait_time: 3s
      - labeldrop:
        - filename
        - stream
      - drop:
          source: job
          drop_counter_reason: "too_many_logs"
          separator: ";"
          value: flog45
      - metrics:
          log_lines_total:
            type: Counter
            description: "total number of log lines"
            prefix: my_promtail_custom_after_drop
            max_idle_duration: 24h
            source: job
            config:
              match_all: true
              action: inc
      - drop:
          source: somesource
          drop_counter_reason: "too_many_logs"
          separator: ";"
          value: somevalue
      - drop:
          source: somesource2
          drop_counter_reason: "just_because_i_can"
          separator: ";"
          value: somevalue2
    kubernetes_sd_configs:
      - role: pod
    relabel_configs:
      - source_labels:
          - __meta_kubernetes_pod_controller_name
        regex: ([0-9a-z-.]+?)(-[0-9a-f]{8,10})?
        action: replace
        target_label: workload
      - action: replace
        source_labels:
        - __meta_kubernetes_pod_name
        target_label: pod
      - action: replace
        source_labels:
        - __meta_kubernetes_pod_container_name
        target_label: container
      - action: replace
        replacement: /var/log/pods/*$1/*.log
        separator: /
        source_labels:
        - __meta_kubernetes_pod_uid
        - __meta_kubernetes_pod_container_name
        target_label: __path__
      - action: replace
        regex: true/(.*)
        replacement: /var/log/pods/*$1/*.log
        separator: /
        source_labels:
        - __meta_kubernetes_pod_annotationpresent_kubernetes_io_config_hash
        - __meta_kubernetes_pod_annotation_kubernetes_io_config_hash
        - __meta_kubernetes_pod_container_name
        target_label: __path__
`

func TestLoadConfig(t *testing.T) {
	p, err := New(sampleConfig)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if p.ScrapeConfigs[0].JobName != "kubernetes-pods" {
		t.Errorf("Expected log level 'kubernetes-pods', got '%s'", p.ScrapeConfigs[0].JobName)
	}
}

func TestToYAML(t *testing.T) {
	p, _ := New(sampleConfig)

	yamlStr, err := p.ToYAML()
	if err != nil {
		t.Fatalf("Failed to convert to YAML: %v", err)
	}

	if yamlStr == "" {
		t.Errorf("Expected non-empty YAML string")
	}
}
func TestAddDropStage(t *testing.T) {
	p, err := New(sampleConfig)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	source := "source1"
	value := "value1"
	p.appendDropStage(source, value, "too_many_logs")

	for _, scrapeConfig := range p.ScrapeConfigs {
		found := false
		for _, stage := range scrapeConfig.PipelineStages {
			if dropStage, ok := stage["drop"].(*DropStage); ok {
				if dropStage.Source == source && dropStage.Value == value {
					found = true
					break
				}
			}
		}
		if !found {
			t.Errorf("Expected drop stage with source '%s' and value '%s' to be added", source, value)
		}
	}
}

func TestParseDropStage(t *testing.T) {
	tests := []struct {
		name     string
		input    map[interface{}]interface{}
		expected *DropStage
	}{
		{
			name: "Valid input with all fields",
			input: map[interface{}]interface{}{
				"source":              "job",
				"drop_counter_reason": "too_many_logs",
				// "separator":           ";",
				"value": "flog45",
			},
			expected: &DropStage{
				Source:            "job",
				DropCounterReason: "too_many_logs",
				// Separator:         ";",
				Value: "flog45",
			},
		},
		{
			name: "Valid input with missing fields",
			input: map[interface{}]interface{}{
				"source": "job",
				"value":  "flog45",
			},
			expected: &DropStage{
				Source:            "job",
				DropCounterReason: "too_many_logs",
				// Separator:         ";",
				Value: "flog45",
			},
		},
		{
			name:     "Empty input",
			input:    map[interface{}]interface{}{},
			expected: nil,
		},
		{
			name: "Invalid field types",
			input: map[interface{}]interface{}{
				"source":              123,
				"drop_counter_reason": true,
				// "separator":           45.67,
				"value": []string{"flog45"},
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _ := parseDropStage(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestRemoveDropStage(t *testing.T) {
	p, _ := New(sampleConfig)
	source := "somesource"
	value := "somevalue"
	p.removeDropStage(value, source)

	for _, scrapeConfig := range p.ScrapeConfigs {
		for _, stage := range scrapeConfig.PipelineStages {
			if _, ok := stage["drop"]; ok {
				// check if the drop stage with source "somesource" and value "somevalue" is removed
				if dropStage, ok := stage["drop"].(*DropStage); ok {
					if dropStage.Source == source && dropStage.Value == value {
						t.Errorf("Expected drop stage with source 'somesource' and value 'somevalue' to be removed")
					}
				}
			}
		}
	}
}

// Fetch all drop stages, then call AllowAllLogs() to remove all drop stages
// and finally check if all drop stages are removed where drop_couter_reason matches "too_many_logs"

func TestAllowAllLogs(t *testing.T) {

	p, _ := New(sampleConfig)
	p.AllowAllLogs()
	dSs, _ := p.extractDropStages()

	// dSs should have only 1 value
	if len(dSs) == 0 {
		t.Errorf("Expected drop stages to be present")
	} else {
		for _, dS := range dSs {
			if dS.DropCounterReason == "too_many_logs" {
				t.Errorf("Expected drop stage with drop_counter_reason 'too_many_logs' to be removed")
			}
		}
	}
}
