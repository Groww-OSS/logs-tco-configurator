package utils

import (
	"configurator/internal/metrics"
	"configurator/internal/models"

	"github.com/rs/zerolog/log"
)

func FindAbusers(currIngestion map[string]int, workloadBudget map[string]int) []string {

	log.Trace().
		Msg("finding abusers...")

	overBudgetWorkloads := make([]string, 0, len(workloadBudget))

	for workload, budget := range workloadBudget {

		log.Info().Str("workload", workload).
			Int("budget", budget).
			Int("ingestion", currIngestion[workload]).
			Msg("budget utilization")

		if currIngestion[workload] > budget {
			overBudgetWorkloads = append(overBudgetWorkloads, workload)
		}
	}
	return overBudgetWorkloads
}

func FindAbusersV2(ingestedBytes []models.WorkloadIngestedBytes, workloadBudget map[string]models.GigaBytes) (overBudgetWorkloads []models.OverBudgetWorkload) {

	log.Trace().
		Msg("finding abusers...")

	for i, w := range ingestedBytes {

		b := workloadBudget[w.Workload]

		if b == 0 {
			continue
		}
		if models.GigaBytes(ingestedBytes[i].Value/1000000000.0) > b {
			overBudgetWorkloads = append(overBudgetWorkloads, models.OverBudgetWorkload{
				Cluster:          w.Cluster,
				Workload:         w.Workload,
				Budget:           b,
				CurrentIngestion: models.GigaBytes(ingestedBytes[i].Value / 1000000000.0),
			})
		}
	}
	return overBudgetWorkloads
}

// Calculate sampling rates based on Ingestion/budget ratio
// to keep Ingestion around the budget
// Sampling will always be between 1 and 100

func CalculateSamplingRates(overBudgetWorkloads []models.OverBudgetWorkload) map[string]float64 {

	// will make it configurable later
	minimum, maximum := 1.0, 100.0

	samplingRates := make(map[string]float64)

	for _, w := range overBudgetWorkloads {

		overBudgetRatio := float64(w.CurrentIngestion) / float64(w.Budget)

		// var samplingPercentage float64

		samplingPercentage := max(minimum, min(maximum, (float64(w.Budget)/float64(w.CurrentIngestion)*100.0)))

		samplingRates[w.Workload] = samplingPercentage

		log.Debug().
			Str("workload", w.Workload).
			Float64("usage_vs_budget_ratio", float64(overBudgetRatio)).
			Float64("budget_gb", float64(w.Budget)).
			Float64("usage_gb", float64(w.CurrentIngestion)).
			Float64("sampling_percentage", samplingPercentage).
			Msg("Calculated sampling rate for workload")

		metrics.RecordSamplingMetrics(
			w.Workload,
			w.Cluster,
			float64(w.CurrentIngestion),
			float64(w.Budget),
			samplingPercentage,
		)

	}

	return samplingRates
}
