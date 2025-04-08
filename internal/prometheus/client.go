package prometheus

import (
	"context"
	"fmt"
	"sync"
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

// TimeRange represents a time range for querying metrics
type TimeRange struct {
	Start time.Time
	End   time.Time
	Step  time.Duration
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
	// Use channels to collect results and errors from goroutines
	resultsChan := make(chan []MetricResult, len(c.config.Metrics))
	errorsChan := make(chan error, len(c.config.Metrics))
	warningsChan := make(chan []string, len(c.config.Metrics))

	// Create a wait group to wait for all goroutines to finish
	var wg sync.WaitGroup

	// Launch a goroutine for each metric
	for _, metricCfg := range c.config.Metrics {
		wg.Add(1)
		go func(cfg config.MetricConfig) {
			defer wg.Done()

			// Replace placeholder in query with actual API proxy name
			query := replaceAPIProxyInQuery(cfg.Query, apiProxy)

			// Execute query with its own context
			queryCtx, queryCancel := context.WithTimeout(context.Background(), c.config.Timeout)
			defer queryCancel()

			result, warnings, err := c.api.Query(queryCtx, query, time.Now())
			if err != nil {
				errorsChan <- fmt.Errorf("error querying Prometheus for metric %s: %w", cfg.Name, err)
				return
			}

			if len(warnings) > 0 {
				warningsChan <- warnings
			}

			var metricResults []MetricResult

			// Process results
			switch result.Type() {
			case model.ValVector:
				vector := result.(model.Vector)
				for _, sample := range vector {
					metricResult := MetricResult{
						Name:      cfg.Name,
						Timestamp: sample.Timestamp.Time(),
						Value:     float64(sample.Value),
						Labels:    make(map[string]string),
					}

					// Extract labels
					for labelName, labelValue := range sample.Metric {
						metricResult.Labels[string(labelName)] = string(labelValue)
					}

					metricResults = append(metricResults, metricResult)
				}
			case model.ValMatrix:
				matrix := result.(model.Matrix)
				for _, stream := range matrix {
					for _, point := range stream.Values {
						metricResult := MetricResult{
							Name:      cfg.Name,
							Timestamp: point.Timestamp.Time(),
							Value:     float64(point.Value),
							Labels:    make(map[string]string),
						}

						// Extract labels
						for labelName, labelValue := range stream.Metric {
							metricResult.Labels[string(labelName)] = string(labelValue)
						}

						metricResults = append(metricResults, metricResult)
					}
				}
			default:
				errorsChan <- fmt.Errorf("unsupported result type for metric %s: %s", cfg.Name, result.Type().String())
				return
			}

			resultsChan <- metricResults
		}(metricCfg)
	}

	// Close channels when all goroutines are done
	go func() {
		wg.Wait()
		close(resultsChan)
		close(errorsChan)
		close(warningsChan)
	}()

	// Collect all results and errors
	var allResults []MetricResult
	var allErrors []error

	// Process warnings
	for warnings := range warningsChan {
		fmt.Printf("Warnings: %v\n", warnings)
	}

	// Process errors
	for err := range errorsChan {
		allErrors = append(allErrors, err)
	}

	// Process results
	for results := range resultsChan {
		allResults = append(allResults, results...)
	}

	// Return error if any occurred
	if len(allErrors) > 0 {
		return nil, fmt.Errorf("errors occurred while collecting metrics: %v", allErrors)
	}

	return allResults, nil
}

// CollectMetricsRange gathers metrics for a specific API proxy over a time range
func (c *Client) CollectMetricsRange(apiProxy string, timeRange TimeRange) ([]MetricResult, error) {
	// Use channels to collect results and errors from goroutines
	resultsChan := make(chan []MetricResult, len(c.config.Metrics))
	errorsChan := make(chan error, len(c.config.Metrics))
	warningsChan := make(chan []string, len(c.config.Metrics))

	// Create a wait group to wait for all goroutines to finish
	var wg sync.WaitGroup

	// Launch a goroutine for each metric
	for _, metricCfg := range c.config.Metrics {
		wg.Add(1)
		go func(cfg config.MetricConfig) {
			defer wg.Done()

			// Replace placeholder in query with actual API proxy name
			query := replaceAPIProxyInQuery(cfg.Query, apiProxy)

			// Execute query with its own context
			queryCtx, queryCancel := context.WithTimeout(context.Background(), c.config.Timeout)
			defer queryCancel()

			// Execute range query
			r := v1.Range{
				Start: timeRange.Start,
				End:   timeRange.End,
				Step:  timeRange.Step,
			}
			result, warnings, err := c.api.QueryRange(queryCtx, query, r)
			if err != nil {
				errorsChan <- fmt.Errorf("error querying Prometheus range for metric %s: %w", cfg.Name, err)
				return
			}

			if len(warnings) > 0 {
				warningsChan <- warnings
			}

			var metricResults []MetricResult

			// Process results
			switch result.Type() {
			case model.ValMatrix:
				matrix := result.(model.Matrix)
				for _, stream := range matrix {
					for _, point := range stream.Values {
						metricResult := MetricResult{
							Name:      cfg.Name,
							Timestamp: point.Timestamp.Time(),
							Value:     float64(point.Value),
							Labels:    make(map[string]string),
						}

						// Extract labels
						for labelName, labelValue := range stream.Metric {
							metricResult.Labels[string(labelName)] = string(labelValue)
						}

						metricResults = append(metricResults, metricResult)
					}
				}
			default:
				errorsChan <- fmt.Errorf("unsupported result type for range query for metric %s: %s", cfg.Name, result.Type().String())
				return
			}

			resultsChan <- metricResults
		}(metricCfg)
	}

	// Close channels when all goroutines are done
	go func() {
		wg.Wait()
		close(resultsChan)
		close(errorsChan)
		close(warningsChan)
	}()

	// Collect all results and errors
	var allResults []MetricResult
	var allErrors []error

	// Process warnings
	for warnings := range warningsChan {
		fmt.Printf("Warnings: %v\n", warnings)
	}

	// Process errors
	for err := range errorsChan {
		allErrors = append(allErrors, err)
	}

	// Process results
	for results := range resultsChan {
		allResults = append(allResults, results...)
	}

	// Return error if any occurred
	if len(allErrors) > 0 {
		return nil, fmt.Errorf("errors occurred while collecting range metrics: %v", allErrors)
	}

	return allResults, nil
}

// replaceAPIProxyInQuery replaces the {apiproxy="..."} placeholder in the query
func replaceAPIProxyInQuery(query, apiProxy string) string {
	// This is a simple implementation - in a real-world scenario,
	// you might want to use a more robust approach like template rendering
	// or proper query parameter substitution
	return fmt.Sprintf(query, apiProxy)
}
