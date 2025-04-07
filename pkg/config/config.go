package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	// Debug mode enables more verbose logging and shorter collection intervals
	Debug bool `yaml:"debug"`

	// APIProxies is a list of API proxy names to collect metrics for
	APIProxies []string `yaml:"apiProxies"`

	// Prometheus configuration
	Prometheus PrometheusConfig `yaml:"prometheus"`

	// Storage configuration
	Storage StorageConfig `yaml:"storage"`
}

// PrometheusConfig contains Prometheus connection settings
type PrometheusConfig struct {
	// URL is the Prometheus server URL
	URL string `yaml:"url"`

	// Timeout for Prometheus API requests
	Timeout time.Duration `yaml:"timeout"`

	// BasicAuth credentials if required
	Username string `yaml:"username,omitempty"`
	Password string `yaml:"password,omitempty"`

	// Metrics is a list of Prometheus metrics to collect
	Metrics []MetricConfig `yaml:"metrics"`
}

// MetricConfig defines a specific Prometheus metric to collect
type MetricConfig struct {
	// Name of the metric
	Name string `yaml:"name"`

	// Query is the PromQL query to execute
	Query string `yaml:"query"`

	// Labels to include with the metric
	Labels []string `yaml:"labels,omitempty"`
}

// StorageConfig contains settings for Parquet file storage
type StorageConfig struct {
	// OutputDir is the directory where Parquet files will be stored
	OutputDir string `yaml:"outputDir"`

	// Compression algorithm to use (snappy, gzip, etc.)
	Compression string `yaml:"compression"`

	// RowGroupSize controls the Parquet row group size
	RowGroupSize int64 `yaml:"rowGroupSize"`
}

// LoadConfig loads the configuration from a YAML file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults
	if cfg.Prometheus.Timeout == 0 {
		cfg.Prometheus.Timeout = 30 * time.Second
	}

	if cfg.Storage.Compression == "" {
		cfg.Storage.Compression = "snappy"
	}

	if cfg.Storage.RowGroupSize == 0 {
		cfg.Storage.RowGroupSize = 128 * 1024 * 1024 // 128MB default
	}

	// Validate required fields
	if cfg.Prometheus.URL == "" {
		return nil, fmt.Errorf("prometheus.url is required")
	}

	if cfg.Storage.OutputDir == "" {
		return nil, fmt.Errorf("storage.outputDir is required")
	}

	if len(cfg.APIProxies) == 0 {
		return nil, fmt.Errorf("at least one API proxy must be specified")
	}

	return &cfg, nil
}
