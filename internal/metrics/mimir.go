package metrics

import (
	"errors"
	"fmt"
	"time"

	"net/http"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

// RoundTrip implements the http.RoundTripper interface
func (rt *HeaderRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("X-Scope-OrgID", rt.OrgID)
	return rt.RoundTripper.RoundTrip(req)
}

// New creates and initializes a new Mimir client
func New(url string, orgId string, queryTimeout time.Duration) (*Mimir, error) {
	if url == "" {
		return nil, errors.New("Mimir URL cannot be empty")
	}
	if orgId == "" {
		return nil, errors.New("Mimir orgId cannot be empty")
	}

	// Create the config with authentication
	config := api.Config{
		Address: url,
		RoundTripper: &HeaderRoundTripper{
			RoundTripper: http.DefaultTransport,
			OrgID:        orgId,
		},
	}

	// Create a new Prometheus API client
	client, err := api.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("error creating Mimir client: %w", err)
	}

	return &Mimir{
		url:          url,
		orgId:        orgId,
		queryTimeout: queryTimeout,
		client:       v1.NewAPI(client),
	}, nil
}

// ToString returns a string representation of the Mimir struct
func (m *Mimir) String() string {
	return fmt.Sprintf("Mimir{url: %s, orgId: %s, queryTimeout: %s}", m.url, m.orgId, m.queryTimeout)
}
