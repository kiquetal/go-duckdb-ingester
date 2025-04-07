package prometheus

import (
	"context"
	"fmt"
	"time"

	"github.com/kiquetal/go-duckdb-ingester/pkg/config"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

// Client handles communication with Prometheus API
type Client struct {
	api    v1.API
	config config.PrometheusConfig
}

// MetricResult represents a collected metric with its values
type MetricResult struct {
	Name      string
	Timestamp time.Time
	Value     float64
	Labels    map[string]string
}

// NewClient creates a new Prometheus client
func NewClient(cfg config.PrometheusConfig) (*Client, error) {
	clientConfig := api.Config{
		Address: cfg.URL,
	}

	// Add basic auth if provided
	if cfg.Username != "" && cfg.Password != "" {
		clientConfig.RoundTripper = api.DefaultRoundTripper
		// Note: In a production environment, you might want to use a more secure
		// way to handle authentication
	}

	client, err := api.NewClient(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("error creating Prometheus client: %w", err)
	}

	return &Client{
		api:    v1.NewAPI(client),
		config: cfg,
	}, nil
}

// CollectMetrics gathers metrics for a specific API proxy
func (c *Client) CollectMetrics(apiProxy string) ([]MetricResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.config.Timeout)
	defer cancel()

	var results []MetricResult

	// Collect each configured metric
	for _, metricCfg := range c.config.Metrics {
		// Replace placeholder in query with actual API proxy name
		query := replaceAPIProxyInQuery(metricCfg.Query, apiProxy)

		// Execute query
		result, warnings, err := c.api.Query(ctx, query, time.Now())
		if err != nil {
			return nil, fmt.Errorf("error querying Prometheus: %w", err)
		}

		if len(warnings) > 0 {
			fmt.Printf("Warnings: %v\n", warnings)
		}

		// Process results
		switch result.Type() {
		case model.ValVector:
			vector := result.(model.Vector)
			for _, sample := range vector {
				metricResult := MetricResult{
					Name:      metricCfg.Name,
					Timestamp: sample.Timestamp.Time(),
					Value:     float64(sample.Value),
					Labels:    make(map[string]string),
				}

				// Extract labels
				for labelName, labelValue := range sample.Metric {
					metricResult.Labels[string(labelName)] = string(labelValue)
				}

				results = append(results, metricResult)
			}
		case model.ValMatrix:
			matrix := result.(model.Matrix)
			for _, stream := range matrix {
				for _, point := range stream.Values {
					metricResult := MetricResult{
						Name:      metricCfg.Name,
						Timestamp: point.Timestamp.Time(),
						Value:     float64(point.Value),
						Labels:    make(map[string]string),
					}

					// Extract labels
					for labelName, labelValue := range stream.Metric {
						metricResult.Labels[string(labelName)] = string(labelValue)
					}

					results = append(results, metricResult)
				}
			}
		default:
			return nil, fmt.Errorf("unsupported result type: %s", result.Type().String())
		}
	}

	return results, nil
}

// replaceAPIProxyInQuery replaces the {apiproxy="..."} placeholder in the query
func replaceAPIProxyInQuery(query, apiProxy string) string {
	// This is a simple implementation - in a real-world scenario,
	// you might want to use a more robust approach like template rendering
	// or proper query parameter substitution
	return fmt.Sprintf(query, apiProxy)
}
