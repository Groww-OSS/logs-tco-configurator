package promtail

import (
	"reflect"
	"testing"
)

func TestNewSamplingStage(t *testing.T) {
	tests := []struct {
		name               string
		format             string
		workload           string
		samplingPercentage float64
		wantStage          *PipelineStage
		wantErr            bool
	}{
		{
			name:               "Valid sampling percentage",
			workload:           "test-workload",
			format:             "{workload=\"%s\"} |= \"\"",
			samplingPercentage: 50.0,
			wantStage: &PipelineStage{
				"match": &MatchStage{
					PipelineName: "automated_sampling",
					Selector:     `{workload="test-workload"} |= ""`,
					Stages: []map[string]Sampling{
						{
							"sampling": {
								Rate: 0.5,
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:               "Valid sampling percentage",
			workload:           "test-workload",
			format:             "{workload=\"%s\", level!=\"info\"} |= \"\"",
			samplingPercentage: 50.0,
			wantStage: &PipelineStage{
				"match": &MatchStage{
					PipelineName: "automated_sampling",
					Selector:     "{workload=\"test-workload\", level!=\"info\"} |= \"\"",
					Stages: []map[string]Sampling{
						{
							"sampling": {
								Rate: 0.5,
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:               "Zero sampling percentage",
			workload:           "test-workload",
			format:             "{workload=\"%s\"} |= \"\"",
			samplingPercentage: 0,
			wantStage: &PipelineStage{
				"match": &MatchStage{
					PipelineName: "automated_sampling",
					Selector:     `{workload="test-workload"} |= ""`,
					Stages: []map[string]Sampling{
						{
							"sampling": {
								Rate: 0.0,
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:               "Full sampling percentage",
			workload:           "test-workload",
			format:             "{workload=\"%s\"} |= \"\"",
			samplingPercentage: 100.0,
			wantStage: &PipelineStage{
				"match": &MatchStage{
					PipelineName: "automated_sampling",
					Selector:     `{workload="test-workload"} |= ""`,
					Stages: []map[string]Sampling{
						{
							"sampling": {
								Rate: 1.0,
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:               "Negative sampling percentage",
			workload:           "test-workload",
			format:             "{workload=\"%s\"} |= \"\"",
			samplingPercentage: -10.0,
			wantStage:          nil,
			wantErr:            true,
		},
		{
			name:               "Sampling percentage exceeds 100",
			workload:           "test-workload",
			format:             "{workload=\"%s\"} |= \"\"",
			samplingPercentage: 150.0,
			wantStage:          nil,
			wantErr:            true,
		},
		{
			name:               "Empty workload name",
			workload:           "",
			format:             "{workload=\"%s\"} |= \"\"",
			samplingPercentage: 50.0,
			wantStage:          nil,
			wantErr:            true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStage, err := newSamplingStage(tt.format, tt.workload, tt.samplingPercentage)

			if (err != nil) != tt.wantErr {
				t.Errorf("newSamplingStage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && !reflect.DeepEqual(gotStage, tt.wantStage) {
				t.Errorf("newSamplingStage() = %v, want %v", gotStage, tt.wantStage)
			}
		})
	}
}

func TestParseSamplingStage(t *testing.T) {
	tests := []struct {
		name                string
		stage               PipelineStage
		format              string
		wantWorkload        string
		wantSamplingPercent float64
		wantErr             bool
	}{
		{
			name: "Valid sampling stage",
			stage: PipelineStage{
				"match": map[interface{}]interface{}{
					"pipeline_name": "automated_sampling",
					"selector":      "{workload=\"test-workload\"} |= \"\"",
					"stages": []interface{}{
						map[interface{}]interface{}{
							"sampling": map[interface{}]interface{}{
								"rate": 0.5,
							},
						},
					},
				},
			},
			format:              "{workload=\"%s\"} |= \"\"",
			wantWorkload:        "test-workload",
			wantSamplingPercent: 50.0,
			wantErr:             false,
		},
		{
			name: "Valid sampling stage with different format",
			stage: PipelineStage{
				"match": map[interface{}]interface{}{
					"pipeline_name": "automated_sampling",
					"selector":      "{workload=\"test-workload\", level!=\"info\"} |= \"\"",
					"stages": []interface{}{
						map[interface{}]interface{}{
							"sampling": map[interface{}]interface{}{
								"rate": 0.1,
							},
						},
					},
				},
			},
			format:              "{workload=\"%s\", level!=\"info\"} |= \"\"",
			wantWorkload:        "test-workload",
			wantSamplingPercent: 10.0,
			wantErr:             false,
		},
		{
			name: "Not a sampling stage - different pipeline name",
			stage: PipelineStage{
				"match": map[interface{}]interface{}{
					"pipeline_name": "different_pipeline",
					"selector":      "{workload=\"test-workload\"} |= \"\"",
					"stages": []interface{}{
						map[interface{}]interface{}{
							"sampling": map[interface{}]interface{}{
								"rate": 0.5,
							},
						},
					},
				},
			},
			format:       "{workload=\"%s\"} |= \"\"",
			wantWorkload: "",
			wantErr:      true,
		},
		{
			name: "Not a match stage",
			stage: PipelineStage{
				"drop": map[interface{}]interface{}{
					"source": "workload",
					"value":  "test-workload",
				},
			},
			format:       "{workload=\"%s\"} |= \"\"",
			wantWorkload: "",
			wantErr:      true,
		},
		{
			name: "Invalid format - no %s placeholder",
			stage: PipelineStage{
				"match": map[interface{}]interface{}{
					"pipeline_name": "automated_sampling",
					"selector":      "{workload=\"test-workload\"} |= \"\"",
					"stages": []interface{}{
						map[interface{}]interface{}{
							"sampling": map[interface{}]interface{}{
								"rate": 0.5,
							},
						},
					},
				},
			},
			format:       "invalid format with no placeholder",
			wantWorkload: "",
			wantErr:      true,
		},
		{
			name: "Missing stages",
			stage: PipelineStage{
				"match": map[interface{}]interface{}{
					"pipeline_name": "automated_sampling",
					"selector":      "{workload=\"test-workload\"} |= \"\"",
				},
			},
			format:       "{workload=\"%s\"} |= \"\"",
			wantWorkload: "",
			wantErr:      true,
		},
		{
			name: "Empty stages",
			stage: PipelineStage{
				"match": map[interface{}]interface{}{
					"pipeline_name": "automated_sampling",
					"selector":      "{workload=\"test-workload\"} |= \"\"",
					"stages":        []interface{}{},
				},
			},
			format:       "{workload=\"%s\"} |= \"\"",
			wantWorkload: "",
			wantErr:      true,
		},
		{
			name: "Missing sampling in stages",
			stage: PipelineStage{
				"match": map[interface{}]interface{}{
					"pipeline_name": "automated_sampling",
					"selector":      "{workload=\"test-workload\"} |= \"\"",
					"stages": []interface{}{
						map[interface{}]interface{}{
							"not_sampling": "something",
						},
					},
				},
			},
			format:       "{workload=\"%s\"} |= \"\"",
			wantWorkload: "",
			wantErr:      true,
		},
		{
			name: "Missing rate in sampling",
			stage: PipelineStage{
				"match": map[interface{}]interface{}{
					"pipeline_name": "automated_sampling",
					"selector":      "{workload=\"test-workload\"} |= \"\"",
					"stages": []interface{}{
						map[interface{}]interface{}{
							"sampling": map[interface{}]interface{}{
								"not_rate": 0.5,
							},
						},
					},
				},
			},
			format:       "{workload=\"%s\"} |= \"\"",
			wantWorkload: "",
			wantErr:      true,
		},
		{
			name: "Rate is not a float64",
			stage: PipelineStage{
				"match": map[interface{}]interface{}{
					"pipeline_name": "automated_sampling",
					"selector":      "{workload=\"test-workload\"} |= \"\"",
					"stages": []interface{}{
						map[interface{}]interface{}{
							"sampling": map[interface{}]interface{}{
								"rate": "not a float",
							},
						},
					},
				},
			},
			format:       "{workload=\"%s\"} |= \"\"",
			wantWorkload: "",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotWorkload, gotSamplingPercent, err := parseSamplingStage(&tt.stage, tt.format)

			if (err != nil) != tt.wantErr {
				t.Errorf("parseSamplingStage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if gotWorkload != tt.wantWorkload {
				t.Errorf("parseSamplingStage() gotWorkload = %v, want %v", gotWorkload, tt.wantWorkload)
			}

			if gotSamplingPercent != tt.wantSamplingPercent {
				t.Errorf("parseSamplingStage() gotSamplingPercent = %v, want %v", gotSamplingPercent, tt.wantSamplingPercent)
			}
		})
	}
}
