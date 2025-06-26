package metrics

import (
	"configurator/internal/models"
	"time"

	"net/http"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

// MetricsQuerier defines the interface for querying metrics
type MetricsQuerier interface {
	GetIngestedGB(cluster string, timeRange string) ([]models.WorkloadIngestedBytes, error)
	GetAvgWorkloadResourceRequest(cluster string, timeRange string) ([]models.WorkloadResourceRequest, error)
}

// Mimir implements the MetricsQuerier interface for Mimir/Prometheus metrics
type Mimir struct {
	url          string
	orgId        string
	queryTimeout time.Duration
	client       v1.API
}

// HeaderRoundTripper adds the X-Scope-OrgID header to each request
type HeaderRoundTripper struct {
	RoundTripper http.RoundTripper
	OrgID        string
}
