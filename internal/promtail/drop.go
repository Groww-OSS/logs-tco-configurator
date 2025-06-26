package promtail

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"
)

func (pCfg *PromtailConfig) DropLogs(newWorkloads []string) (isConfigUpdated bool) {

	isConfigUpdated = false

	// Check if the workloads are already dropped
	alreadyDroppedWorkloads, err := pCfg.getDroppedWorkloads()
	if err != nil {
		log.Error().
			Caller().
			Msg("failed to get already dropped workloads")
		return
	}

	// Create a map for quick lookup of already dropped workloads
	droppedWorkloadsMap := make(map[string]struct{})

	for _, droppedWorkload := range alreadyDroppedWorkloads {
		droppedWorkloadsMap[droppedWorkload] = struct{}{}
	}

	// Iterate through new workloads and drop only those not already dropped
	for _, workload := range newWorkloads {

		if _, exists := droppedWorkloadsMap[workload]; exists {

			log.Debug().
				Str("workload", workload).
				Msg("will not add drop stage, as logs are already dropped")
			continue

		}

		log.Debug().
			Str("workload", workload).
			Msg("dropping logs")

		pCfg.appendDropStage("workload", workload, "too_many_logs")

		isConfigUpdated = true
	}
	return
}

// Parses a drop stage from a map to a DropStage struct.
// Handle missing values
func parseDropStage(m map[interface{}]interface{}) (*DropStage, error) {

	var source, value, dropCounterReason string

	if s, ok := m["source"].(string); ok {
		source = s
	} else {
		return nil, errors.New("can't parse drop stage, source field is missing or not a string")
	}

	if dcr, ok := m["drop_counter_reason"].(string); ok {
		dropCounterReason = dcr
	} else {
		dropCounterReason = "too_many_logs"
	}

	if v, ok := m["value"].(string); ok {
		value = v
	} else {
		return nil, errors.New("can't parse drop stage, value field is missing or not a string")
	}

	// return newDropStage(source, value, dropCounterReason)
	return &DropStage{
		Source:            source,
		DropCounterReason: dropCounterReason,
		Value:             value,
		// Separator:         separator,
	}, nil
}

// extractDropStages return all drop stages from the pipeline stages.
func (pCfg *PromtailConfig) extractDropStages() ([]*DropStage, error) {

	var dropStages []*DropStage

	// Iterate over all scrape configs and pipeline stages.
	for _, scrapeConfig := range pCfg.ScrapeConfigs {

		for _, stage := range scrapeConfig.PipelineStages {

			// Check if the stage is a drop stage and print the drop stage.
			if dropStage, ok := stage["drop"]; ok {

				// Check if the drop stage can be asserted to the DropStage type.
				switch dst := dropStage.(type) {

				case map[interface{}]interface{}:

					convertedDropStage, err := parseDropStage(dst)
					if err != nil {
						log.Error().
							Caller().
							Msg("Failed to parse drop stage")
					}

					dropStages = append(dropStages, convertedDropStage)

				case *DropStage:

					fmt.Printf("drop PipelineStage: \n%v\n", *dst)
					dropStages = append(dropStages, dst)

				default:

					return nil, errors.New("failed to assert type *DropStage or map[interface{}]interface{} for drop stage")

				}

			}

		}
	}

	return dropStages, nil
}

// Get already dropped workloads
func (pCfg *PromtailConfig) getDroppedWorkloads() ([]string, error) {
	var droppedWorkloads []string
	dropStages, err := pCfg.extractDropStages()
	if err != nil {
		log.Error().
			Caller().
			Msg("Failed to extract drop pipeline stages")
		return nil, err
	}
	for _, dropStage := range dropStages {
		if dropStage.Source == "workload" && dropStage.DropCounterReason == "too_many_logs" {
			droppedWorkloads = append(droppedWorkloads, dropStage.Value)
		}
	}
	return droppedWorkloads, nil
}

// AddDropStage adds a new drop stage to the pipeline stages.
func (pCfg *PromtailConfig) appendDropStage(source string, value string, reason string) {

	log.Trace().
		Str("source", source).
		Str("value", value).
		Str("reason", reason).
		Msg("adding DropStage")

	newDropStageMap := &DropStage{
		Source:            source,
		DropCounterReason: reason,
		Value:             value,
		// Separator:         ";",
	}

	for i, scrapeConfig := range pCfg.ScrapeConfigs {
		dropStage := PipelineStage{"drop": newDropStageMap}
		pCfg.ScrapeConfigs[i].PipelineStages = append(scrapeConfig.PipelineStages, dropStage)
	}

}

// removeDropStage remove a drop stage to the pipeline stages by source and value.
func (p *PromtailConfig) removeDropStage(source string, value string) {
	log.Info().Msg(fmt.Sprintf("Removing DropStage from promtail config: source=%s, value=%s\n", source, value))

	for i, scrapeConfig := range p.ScrapeConfigs {

		newPipelineStages := []PipelineStage{}

		for _, stage := range scrapeConfig.PipelineStages {

			if dropStage, ok := stage["drop"]; ok {

				switch ds := dropStage.(type) {

				case map[interface{}]interface{}:

					convertedDropStage, err := parseDropStage(ds)
					if err != nil {
						log.Error().Caller().Msg("Failed to parse drop stage")
					}
					if convertedDropStage.Source != source || convertedDropStage.Value != value {
						newPipelineStages = append(newPipelineStages, stage)
					}

				case *DropStage:

					if ds.Source != source || ds.Value != value {
						newPipelineStages = append(newPipelineStages, stage)
					}

				default:
					log.Error().
						Caller().
						Msg("Failed to assert type *DropStage or map[interface{}]interface{} for drop stage")
				}
			} else {
				newPipelineStages = append(newPipelineStages, stage)
			}
		}
		p.ScrapeConfigs[i].PipelineStages = newPipelineStages
	}
}

func (pCfg *PromtailConfig) AllowLogs(workloads []string) {

	log.Info().
		Str("workloads", fmt.Sprintf("%+v", workloads)).
		Msg("allowing logs for workloads")

	for _, workload := range workloads {
		pCfg.removeDropStage("workload", workload)
	}

}

// AllowAllLogs will removes all automatically added drop stages from the pipeline stages.
// It will remove staged where reason matches "too_many_logs"
func (p *PromtailConfig) AllowAllLogs() error {

	log.Trace().
		Msg("Removing all DropStages")

	for i, scrapeConfig := range p.ScrapeConfigs {

		newPipelineStages := []PipelineStage{}

		for _, stage := range scrapeConfig.PipelineStages {

			if dropStage, ok := stage["drop"]; ok {

				switch ds := dropStage.(type) {

				case map[interface{}]interface{}:

					convertedDropStage, err := parseDropStage(ds)
					if err != nil {
						log.Error().Caller().Msg("Failed to parse drop stage")
					}
					if convertedDropStage.DropCounterReason != "too_many_logs" {
						newPipelineStages = append(newPipelineStages, stage)
					}

				case *DropStage:

					if ds.DropCounterReason != "too_many_logs" {
						newPipelineStages = append(newPipelineStages, stage)
					}

				default:

					log.Error().
						Caller().
						Msg("Failed to assert type *DropStage or map[interface{}]interface{} for drop stage")

				}

			} else {
				newPipelineStages = append(newPipelineStages, stage)
			}

		}

		p.ScrapeConfigs[i].PipelineStages = newPipelineStages

	}

	return nil

}
