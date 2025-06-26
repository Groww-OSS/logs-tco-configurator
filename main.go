// Disclaimer: This code shared in this OSS repository is derived from Groww's internal Configurator application.
// It may not be identical to what's running in Groww's production environment and might not be synchronized
// with our latest internal changes. This is provided for reference purposes only.

// Package main provides the entry point for the Configurator application
// which monitors and manages log ingestion budgets for workloads in Kubernetes

package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"

	"configurator/config"
	"configurator/internal/budget"
	"configurator/internal/kubernetes"
	"configurator/internal/logger"
	"configurator/internal/metrics"
	"configurator/internal/models"
	"configurator/internal/promtail"
	"configurator/internal/utils"
)

// Application constants
const (
	timeRange      = "24h"
	retryDelaySecs = 30
)

var (
	cfg           *config.Config
	k8sClient     *kubernetes.K8sClient
	mimirClient   *metrics.Mimir
	budgetConfig  budget.Budget
	cronMutex     sync.Mutex
	cronScheduler *cron.Cron
	metricsPort   = flag.String("metrics-port", "9091", "Port to expose Prometheus metrics on")
)

func main() {

	initConfig()
	initLogger()

	// Run initialization tasks in parallel
	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		initKubernetes()
	}()

	go func() {
		defer wg.Done()
		initBudget()
	}()

	go func() {
		defer wg.Done()
		initMetrics()
	}()

	go startMetricsServer()

	// Wait for all initialization tasks to complete
	wg.Wait()
	log.Info().Msg("All initialization tasks completed successfully")

	startScheduler()

	handleShutdown()
}

func initConfig() {
	var err error
	cfg, err = config.Init()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}
}

func initLogger() {
	logger.InitLogger(cfg.Log.Level, cfg.Log.Format)
	log.Debug().
		Msg("Logger initialized successfully")
}

func initKubernetes() {
	var err error
	k8sClient, err = kubernetes.New(cfg.KubeConfig)
	if err != nil {
		log.Fatal().
			Err(err).
			Msg("Failed to create Kubernetes client")
	}
	log.Debug().
		Msg("Kubernetes client initialized successfully")
}

// initBudget loads the budget configuration
func initBudget() {
	var err error
	budgetConfig, err = budget.New(cfg.Budget.ConfigPath)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load budget configuration")
	}
	log.Info().Msg("Budget configuration loaded successfully")
}

// initMetrics initializes the metrics client
func initMetrics() {
	var err error
	mimirClient, err = metrics.New(
		cfg.Metrics.MimirEndpoint,
		cfg.Metrics.MimirTenant,
		cfg.Metrics.QueryTimeout,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create Mimir client")
	}
	log.Info().Msg("Metrics client initialized successfully")
}

// handleShutdown sets up signal handling for graceful shutdown
func handleShutdown() {
	// Set up channel to catch signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Block until we receive a signal
	sig := <-sigChan
	log.Info().Str("signal", sig.String()).Msg("Received shutdown signal")

	if cronScheduler != nil {
		log.Info().Msg("Stopping scheduler...")
		cronScheduler.Stop()
	}
	log.Info().Msg("Shutdown complete")
	os.Exit(0)
}

func startScheduler() {
	log.Info().Msg("Starting scheduler...")

	// Load time zone
	location, err := time.LoadLocation(cfg.Scheduling.TimeZone)
	if err != nil {
		log.Fatal().Err(err).
			Msg("💀 Failed to load time zone")
	}
	log.Debug().
		Str("timezone", location.String()).
		Msg("Loaded time zone")

	// Use configured time zone for the cron scheduler
	cronScheduler = cron.New(cron.WithLocation(location))

	cronScheduler.AddFunc(
		cfg.Scheduling.Cron.BudgetReset,
		func() {
			cronMutex.Lock()
			defer cronMutex.Unlock()
			midnightCron()
		})

	cronScheduler.Start()

	log.Info().
		Str("quota_reset", cfg.Scheduling.Cron.BudgetReset).
		Msg("Scheduler started with cron jobs")
}

// midnightCron is the main job that runs at the configured schedule to check workload
// ingestion and apply sampling if needed
func midnightCron() {
	log.Debug().
		Msg("Starting daily budget check and sampling adjustment")

	// Step 1: Get budgets and current ingestion data
	workloadBudgets, workloadResources, ingestedBytes, err := collectBudgetData()
	if err != nil {
		log.Error().Err(err).Msg("Budget check failed")
		metrics.RecordTaskExecution(false)
		return
	}

	// Step 2: Calculate dynamic budgets based on resource usage
	dynamicBudget, err := calculateDynamicBudgets(workloadBudgets, workloadResources)
	if err != nil {
		log.Error().Err(err).Msg("Failed to calculate dynamic budgets")
		metrics.RecordTaskExecution(false)
		return
	}

	// Step 3: Find workloads exceeding their budget
	overBudgetWorkloads := findOverBudgetWorkloads(ingestedBytes, dynamicBudget)
	if len(overBudgetWorkloads) == 0 {
		log.Info().Msg("no workloads are currently over budget")
	}

	// Step 4: Apply sampling to over-budget workloads
	err = applySamplingToWorkloads(overBudgetWorkloads)
	if err != nil {
		log.Error().Err(err).Msg("Failed to apply sampling")
		metrics.RecordTaskExecution(false)
		return
	}
	metrics.RecordTaskExecution(true)

	log.Info().
		Msg("Daily budget check and sampling adjustment completed successfully")
}

// collectBudgetData gathers all necessary data for budget calculations
func collectBudgetData() (map[string]models.GigaBytes, []models.WorkloadResourceRequest, []models.WorkloadIngestedBytes, error) {
	var wg sync.WaitGroup
	wg.Add(3)

	// Use channels for results and errors
	workloadBudgetCh := make(chan map[string]models.GigaBytes, 1)
	workloadResourceCh := make(chan []models.WorkloadResourceRequest, 1)
	ingestedBytesCh := make(chan []models.WorkloadIngestedBytes, 1)
	errCh := make(chan error, 3)

	// Get configured workload budgets concurrently
	go func() {
		defer wg.Done()
		budgets, err := budgetConfig.ExtractBudget(cfg.Budget.Org, cfg.Budget.Env)
		if err != nil {
			errCh <- fmt.Errorf("failed to extract budget: %w", err)
			return
		}
		workloadBudgetCh <- budgets
	}()

	// Get resource requests for workloads concurrently
	go func() {
		defer wg.Done()
		resources, err := mimirClient.GetAvgWorkloadResourceRequest(cfg.Cluster, timeRange)
		if err != nil {
			errCh <- fmt.Errorf("failed to get resource requests: %w", err)
			return
		}
		workloadResourceCh <- resources
	}()

	// Get current ingestion data concurrently
	go func() {
		defer wg.Done()
		ingested, err := mimirClient.GetIngestedGB(cfg.Cluster, timeRange)
		if err != nil {
			errCh <- fmt.Errorf("failed to get current ingestion: %w", err)
			return
		}
		ingestedBytesCh <- ingested
	}()

	// Wait for all goroutines to complete
	wg.Wait()
	close(errCh)

	// Check for errors
	for err := range errCh {
		if err != nil {
			return nil, nil, nil, err
		}
	}

	// Collect results
	var workloadBudgetOverride map[string]models.GigaBytes
	var workloadResourceRequests []models.WorkloadResourceRequest
	var ingestedBytes []models.WorkloadIngestedBytes

	// Non-blocking receive from channels
	select {
	case workloadBudgetOverride = <-workloadBudgetCh:
	default:
		return nil, nil, nil, fmt.Errorf("budget data collection failed")
	}

	select {
	case workloadResourceRequests = <-workloadResourceCh:
	default:
		return nil, nil, nil, fmt.Errorf("resource request data collection failed")
	}

	select {
	case ingestedBytes = <-ingestedBytesCh:
	default:
		return nil, nil, nil, fmt.Errorf("ingested bytes data collection failed")
	}

	return workloadBudgetOverride, workloadResourceRequests, ingestedBytes, nil
}

// calculateDynamicBudgets computes the budget for each workload based on resource usage
func calculateDynamicBudgets(
	workloadBudgetOverride map[string]models.GigaBytes,
	workloadResourceRequests []models.WorkloadResourceRequest,
) (map[string]models.GigaBytes, error) {
	return budget.CalculateDynamicBudget(
		workloadResourceRequests,
		workloadBudgetOverride,
		cfg.Budget.Multiplier,
		cfg.Budget.Minimum,
	)
}

// findOverBudgetWorkloads identifies workloads that are exceeding their budget
func findOverBudgetWorkloads(
	ingestedBytes []models.WorkloadIngestedBytes,
	dynamicBudget map[string]models.GigaBytes,
) []models.OverBudgetWorkload {
	overBudgetWorkloads := utils.FindAbusersV2(ingestedBytes, dynamicBudget)

	if len(overBudgetWorkloads) > 0 {
		log.Info().
			Int("count", len(overBudgetWorkloads)).
			Msg("Found workloads over budget")
	}

	return overBudgetWorkloads
}

// applySamplingToWorkloads configures sampling for workloads that exceed their budget
func applySamplingToWorkloads(overBudgetWorkloads []models.OverBudgetWorkload) error {
	// Calculate sampling rates
	samplingRates := utils.CalculateSamplingRates(overBudgetWorkloads)

	// Get current Promtail config
	promtailConfig, err := getPromtailConfig()
	if err != nil {
		return err
	}

	// Apply sampling configuration
	if err := updateSamplingConfig(promtailConfig, samplingRates); err != nil {
		return err
	}

	return nil
}

// getPromtailConfig retrieves and parses the current Promtail configuration
func getPromtailConfig() (*promtail.PromtailConfig, error) {
	// Fetch Promtail config
	configYaml, err := k8sClient.FetchSecretValue(
		cfg.Promtail.Secret.Namespace,
		cfg.Promtail.Secret.Name,
		cfg.Promtail.Secret.Key,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Promtail config: %w", err)
	}

	// Parse the config
	config, err := promtail.New(configYaml)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Promtail config: %w", err)
	}

	return config, nil
}

// updateSamplingConfig updates the Promtail configuration with new sampling rates
func updateSamplingConfig(p *promtail.PromtailConfig, samplingRates map[string]float64) error {
	// Get current sampled workloads for tracking/notification
	sampledWorkloadsMap, err := p.GetSampledWorkloads(cfg.Promtail.Sampling.Selector.Format)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get current sampled workloads")
		// Continue despite this error
	} else {
		sampledWorkloads := make([]string, 0, len(sampledWorkloadsMap))
		for workload := range sampledWorkloadsMap {
			sampledWorkloads = append(sampledWorkloads, workload)
		}

		if len(sampledWorkloads) == 0 {
			log.Info().
				Msg("there are no previously sampled workloads")
		}
		log.Info().
			Strs("workloads", sampledWorkloads).
			Msg("resetting sampling for previously sampled workloads")
	}

	// Remove all existing sampling stages
	if _, err := p.RemoveAllSamplingStages(cfg.Promtail.Sampling.Selector.Format); err != nil {
		return fmt.Errorf("failed to remove existing sampling stages: %w", err)
	}

	// Add new sampling stages
	_ = p.AddSamplingStages(samplingRates, cfg.Promtail.Sampling.Selector.Format)

	// Validate the updated config
	if err := p.ValidateConfig(cfg.Promtail.LocalBin); err != nil {
		return fmt.Errorf("promtail config validation failed: %w", err)
	}

	// Convert the config to YAML
	yamlContent, err := p.ToYAML()
	if err != nil {
		return fmt.Errorf("failed to convert Promtail config to YAML: %w", err)
	}

	// // Update the Promtail secret

	if err := k8sClient.UpdateSecretValue(
		cfg.Promtail.Secret.Namespace,
		cfg.Promtail.Secret.Name,
		cfg.Promtail.Secret.Key,
		yamlContent,
		cfg.DryRun,
	); err != nil {
		return fmt.Errorf("failed to update Promtail config secret: %w", err)
	}

	log.Debug().
		Msg("Successfully updated Promtail configuration with new sampling rates")
	return nil
}

// startMetricsServer starts an HTTP server to expose Prometheus metrics
func startMetricsServer() {
	http.Handle("/metrics", promhttp.Handler())
	log.Info().
		Str("port", *metricsPort).
		Msg("Starting metrics server")
	if err := http.ListenAndServe(":"+*metricsPort, nil); err != nil {
		log.Error().Err(err).Msg("Failed to start metrics server")
	}
}
