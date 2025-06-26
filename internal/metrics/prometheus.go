package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// MetricsPrefix is the global prefix for all metrics
const (
	MetricsPrefix = "tco_configurator_"
)

var (
	// Register metrics for cron job execution tracking with status label
	cronExecutionCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: MetricsPrefix + "task_executions_total",
			Help: "Total number of task executions",
		},
		[]string{"status"},
	)

	// SamplingMetrics tracks sampling information for workloads
	samplingMetrics = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: MetricsPrefix + "log_sampling_info",
			Help: "Information about log sampling including current ingestion, budget and sampling percentage",
		},
		[]string{"workload", "cluster", "metric_type"},
	)
)

// RecordSamplingMetrics records sampling metrics for a workload
func RecordSamplingMetrics(workload, cluster string, currentIngestion, budget, samplingPercentage float64) {
	samplingMetrics.WithLabelValues(workload, cluster, "current_ingestion").Set(currentIngestion)
	samplingMetrics.WithLabelValues(workload, cluster, "daily_budget").Set(budget)
	samplingMetrics.WithLabelValues(workload, cluster, "sampling_percentage").Set(samplingPercentage)
}

// RecordTaskExecution records the execution of the task job
func RecordTaskExecution(success bool) {
	if success {
		cronExecutionCount.WithLabelValues("success").Inc()
	} else {
		cronExecutionCount.WithLabelValues("failure").Inc()
	}
}
