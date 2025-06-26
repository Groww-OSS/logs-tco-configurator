package promtail

import (
	"errors"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
)

var errNotASamplingStage = errors.New("not a sampling stage")

func newSamplingStage(format string, workload string, samplingPercentage float64) (*PipelineStage, error) {
	// Check if the sampling percentage is valid
	if samplingPercentage < 0 || samplingPercentage > 100 {
		return nil, NewOutOfRangePercentageError(samplingPercentage)
	}

	if workload == "" {
		return nil, NewCanNotCreateSamplingStageError("workload name can not be empty")
	}

	return &PipelineStage{
		"match": &MatchStage{
			PipelineName: "automated_sampling",
			Selector:     fmt.Sprintf(format, workload),
			Stages: []map[string]Sampling{
				{
					"sampling": {
						Rate: float64(samplingPercentage) / 100.0, // Convert percentage to a fraction
					},
				},
			},
		},
	}, nil

}

// Parses a sampling stage from a map to a Sampling struct.
// It returns the workload and sampling percentage.
// If the stage is not a match stage or if the sampling stage is not found, it returns an error.
func parseSamplingStage(s *PipelineStage, format string) (workload string, samplingPercentage float64, err error) {

	// Check if the stage is a match stage

	// extract the prefix and suffix from the format string
	idx := strings.Index(format, "%s")
	if idx == -1 {
		return "", 0, fmt.Errorf("invalid format string: %s", format)
	}
	prefix, suffix := format[:idx], format[idx+2:]

	if matchStage, ok := (*s)["match"]; ok {

		switch m := matchStage.(type) {
		case map[interface{}]interface{}:

			if pipelineName, ok := m["pipeline_name"].(string); ok && pipelineName == "automated_sampling" {
				if selector, ok := m["selector"].(string); ok && len(selector) > len(prefix) {

					// Extract workload from selector
					if startIdx, endIdx := len(prefix), len(selector)-len(suffix); endIdx > startIdx {
						workload = selector[startIdx:endIdx]

						// Process stages to find sampling rate
						if stages, ok := m["stages"].([]interface{}); ok && len(stages) >= 1 {
							if stage0, ok := stages[0].(map[interface{}]interface{}); ok {
								if samplingMap, ok := stage0["sampling"].(map[interface{}]interface{}); ok {
									if rate, ok := samplingMap["rate"].(float64); ok {
										samplingPercentage = rate * 100.0
										return workload, samplingPercentage, nil
									}
									return "", 0, fmt.Errorf("sampling rate is not a float64")
								}
								return "", 0, fmt.Errorf("sampling field not found in stage")
							}
							return "", 0, fmt.Errorf("invalid stage format")
						}
						return "", 0, fmt.Errorf("stages missing or empty")
					}
					return "", 0, fmt.Errorf("failed to extract workload from selector: %v", selector)
				}
			}
		}
	}
	return "", 0, errNotASamplingStage
}

func (p *PromtailConfig) AddSamplingStages(newWorkloads map[string]float64, format string) (isConfigUpdated bool) {

	// check if newWorkloads is empty
	if len(newWorkloads) == 0 {
		log.Debug().Caller().Msg("no new workloads to add")
		return false
	}

	isConfigUpdated = false
	log.Info().
		Str("workloads", fmt.Sprintf("%v", newWorkloads)).
		Msg("adding new sampling stages")

	for i := range p.ScrapeConfigs {
		for w, s := range newWorkloads {
			s, err := newSamplingStage(format, w, s)

			if err != nil {
				log.Error().Err(err).Msg(fmt.Sprintf("failed to create sampling stage: %v", err))
				continue
			}
			p.ScrapeConfigs[i].PipelineStages = append(p.ScrapeConfigs[i].PipelineStages, *s)
			isConfigUpdated = true
		}
	}
	return
}

// RemoveAllSamplingStages removes all sampling stages from the pipeline stages in each scrape config.
// It iterates through all scrape configurations and their pipeline stages, identifying sampling stages
// and excluding them from the new set of pipeline stages.

func (p *PromtailConfig) RemoveAllSamplingStages(format string) (isConfigUpdated bool, err error) {

	log.Debug().Msg("removing all existing sampling stages")

	isConfigUpdated = false

	for i, scrapeConfig := range p.ScrapeConfigs {

		newPipelineStages := []PipelineStage{}

		for _, stage := range scrapeConfig.PipelineStages {

			_, _, err := parseSamplingStage(&stage, format)

			if err == nil { // This is a sampling stage, so we skip it
				continue
			} else if errors.Is(err, errNotASamplingStage) {
				newPipelineStages = append(newPipelineStages, stage) // This is not a sampling stage, so we keep it
				isConfigUpdated = true
			} else {
				log.Error().Err(err).Msg(fmt.Sprintf("failed to parse sampling stage: %+v", stage))
				return false, err
			}
		}
		p.ScrapeConfigs[i].PipelineStages = newPipelineStages
	}
	return isConfigUpdated, nil
}

// GetSampledWorkloads returns a map of workload names to their sampling percentages
// Useful for reporting which workloads are being sampled
func (p *PromtailConfig) GetSampledWorkloads(format string) (map[string]float64, error) {

	sampledWorkloads := make(map[string]float64)

	for _, scrapeConfig := range p.ScrapeConfigs {
		for _, stage := range scrapeConfig.PipelineStages {

			workload, samplingPercentage, err := parseSamplingStage(&stage, format)

			if err == nil {
				sampledWorkloads[workload] = samplingPercentage

			} else if !errors.Is(err, errNotASamplingStage) {
				log.Error().Err(err).Msg(fmt.Sprintf("failed to parse sampling stage: %+v", stage))
				return nil, err
			}
		}
	}

	return sampledWorkloads, nil
}
