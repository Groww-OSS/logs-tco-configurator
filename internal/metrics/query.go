package metrics

import (
	"configurator/internal/models"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v4"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/rs/zerolog/log"
)

// Constants for metric names and defaults
const (
	logBytesMetric              = "promtail_custom_processed_log_bytes_total"
	workloadCPURequestMetric    = "workload_cpu_request"
	workloadMemoryRequestMetric = "workload_memory_request"
	defaultTimeRange            = "24h"
)

// query executes a PromQL query against the Mimir instance with retry logic
func (m *Mimir) query(query string) (model.Value, error) {
	log.Trace().
		Str("query", query).
		Msg("Querying Mimir")

	var result model.Value
	var warnings v1.Warnings

	operation := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), m.queryTimeout)
		defer cancel()

		log.Debug().Str("timeout", m.queryTimeout.String()).Msg("Querying Mimir with timeout")
		var err error
		result, warnings, err = m.client.Query(ctx, query, time.Now(), v1.WithTimeout(m.queryTimeout))

		if err != nil {
			log.Error().Err(err).Msg("Error querying Mimir")
			return err
		}
		if len(warnings) > 0 {
			log.Warn().Strs("warnings", warnings).Msg("Warnings from Mimir query")
		}
		return nil
	}

	// Use exponential backoff for retries
	expBackoff := backoff.NewExponentialBackOff()
	expBackoff.InitialInterval = 1 * time.Second
	expBackoff.MaxElapsedTime = 1 * time.Hour
	expBackoff.Multiplier = 1.2

	startTime := time.Now()

	err := backoff.RetryNotify(
		operation,
		expBackoff,
		func(err error, duration time.Duration) {
			log.Warn().Err(err).Dur("retry_in", duration).Msg("Query failed, will retry")
		})

	if err != nil {
		return nil, fmt.Errorf("failed to query Mimir after retries: %w", err)
	}

	log.Trace().Dur("duration", time.Since(startTime)).Msg("Query completed")

	return result, nil
}

// GetIngestedGB retrieves the ingested gigabytes for all workloads in a cluster
func (m *Mimir) GetIngestedGB(cluster string, timeRange string) ([]models.WorkloadIngestedBytes, error) {
	log.Trace().Msg("Fetching total ingestion for workloads")

	if cluster == "" {
		return nil, errors.New("cluster cannot be empty")
	}

	if timeRange == "" {
		log.Info().Msg("timeRange is empty, using default timeRange")
		timeRange = defaultTimeRange
	}

	q := fmt.Sprintf(
		"sum by (cluster, workload) (increase(%s{cluster=~'%s'}[%s]))",
		logBytesMetric,
		cluster,
		timeRange,
	)

	result, err := m.query(q)
	if err != nil {
		return nil, fmt.Errorf("failed to query Mimir: %w", err)
	}

	log.Debug().Str("resultType", fmt.Sprintf("%T", result)).Msg("Parsing query result")

	var ingestedBytesList []models.WorkloadIngestedBytes

	matrix, ok := result.(model.Vector)
	if !ok {
		return nil, fmt.Errorf("expected Vector result but got %T", result)
	}

	for _, sample := range matrix {
		ingestedBytesList = append(ingestedBytesList, models.WorkloadIngestedBytes{
			Cluster:  string(sample.Metric["cluster"]),
			Workload: string(sample.Metric["workload"]),
			Value:    float64(sample.Value),
		})
	}

	return ingestedBytesList, nil
}

// GetAvgWorkloadResourceRequest retrieves the average CPU and memory requests for workloads
func (m *Mimir) GetAvgWorkloadResourceRequest(cluster string, timeRange string) ([]models.WorkloadResourceRequest, error) {
	if cluster == "" {
		return nil, errors.New("cluster cannot be empty")
	}

	if timeRange == "" {
		log.Info().Msg("timeRange is empty, using default timeRange")
		timeRange = defaultTimeRange
	}

	// Query CPU requests
	cpuQuery := fmt.Sprintf(
		"sum by (cluster, workload) (avg_over_time(%s{cluster=~'%s'}[%s]))",
		workloadCPURequestMetric,
		cluster,
		timeRange,
	)

	// Query memory requests
	memQuery := fmt.Sprintf(
		"sum by (cluster, workload) (avg_over_time(%s{cluster=~'%s'}[%s]))",
		workloadMemoryRequestMetric,
		cluster,
		timeRange,
	)

	// Execute both queries
	cpuResult, err := m.query(cpuQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to query CPU metrics: %w", err)
	}

	memResult, err := m.query(memQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to query memory metrics: %w", err)
	}

	// Type checking and conversion
	cpuVector, ok := cpuResult.(model.Vector)
	if !ok {
		return nil, fmt.Errorf("expected Vector result for CPU but got %T", cpuResult)
	}

	memVector, ok := memResult.(model.Vector)
	if !ok {
		return nil, fmt.Errorf("expected Vector result for memory but got %T", memResult)
	}

	// Map results to structures
	cpuRequest := make(map[string]models.Cores)
	memoryRequest := make(map[string]models.Bytes)

	for _, sample := range memVector {
		memoryRequest[string(sample.Metric["workload"])] = models.Bytes(sample.Value)
	}

	for _, sample := range cpuVector {
		cpuRequest[string(sample.Metric["workload"])] = models.Cores(sample.Value)
	}

	// Combine results into workload resources
	var workloadsResources []models.WorkloadResourceRequest
	for workload, cpu := range cpuRequest {
		workloadResource := models.WorkloadResourceRequest{
			Cluster:  cluster,
			Workload: workload,
			CPU:      cpu,
			Memory:   memoryRequest[workload],
		}
		workloadsResources = append(workloadsResources, workloadResource)
	}

	return workloadsResources, nil
}
