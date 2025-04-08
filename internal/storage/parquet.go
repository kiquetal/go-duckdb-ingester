package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"time" // Used for time.Time methods on metric.Timestamp

	"github.com/kiquetal/go-duckdb-ingester/internal/prometheus"
	"github.com/kiquetal/go-duckdb-ingester/pkg/config"
	"github.com/xitongsys/parquet-go-source/local"
	"github.com/xitongsys/parquet-go/parquet"
	"github.com/xitongsys/parquet-go/writer"
)

// MetricRecord represents a row in the Parquet file
type MetricRecord struct {
	Timestamp  int64             `parquet:"name=timestamp, type=INT64, convertedtype=TIMESTAMP_MILLIS"`
	MetricName string            `parquet:"name=metric_name, type=BYTE_ARRAY, convertedtype=UTF8"`
	Value      float64           `parquet:"name=value, type=DOUBLE"`
	ApiProxy   string            `parquet:"name=api_proxy, type=BYTE_ARRAY, convertedtype=UTF8"`
	Labels     map[string]string `parquet:"name=labels, type=MAP, convertedtype=MAP, keytype=BYTE_ARRAY, keyconvertedtype=UTF8, valuetype=BYTE_ARRAY, valueconvertedtype=UTF8"`
	Date       string            `parquet:"name=date, type=BYTE_ARRAY, convertedtype=UTF8"`
}

// ParquetStorage handles storing metrics in Parquet files
type ParquetStorage struct {
	config config.StorageConfig
}

// NewParquetStorage creates a new Parquet storage handler
func NewParquetStorage(cfg config.StorageConfig) (*ParquetStorage, error) {
	// Ensure output directory exists
	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	return &ParquetStorage{
		config: cfg,
	}, nil
}

// StoreMetrics stores the collected metrics in a Parquet file
func (s *ParquetStorage) StoreMetrics(metrics []prometheus.MetricResult, filename string) error {
	// Ensure directory exists
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory for Parquet file: %w", err)
	}

	// Create Parquet file writer
	fw, err := local.NewLocalFileWriter(filename)
	if err != nil {
		return fmt.Errorf("failed to create Parquet file: %w", err)
	}
	defer fw.Close()

	// Configure Parquet writer
	pw, err := writer.NewParquetWriter(fw, new(MetricRecord), int64(s.config.RowGroupSize))
	if err != nil {
		return fmt.Errorf("failed to create Parquet writer: %w", err)
	}

	// Set compression
	compressionType := parquet.CompressionCodec_SNAPPY // Default
	switch s.config.Compression {
	case "gzip":
		compressionType = parquet.CompressionCodec_GZIP
	case "lz4":
		compressionType = parquet.CompressionCodec_LZ4
	case "zstd":
		compressionType = parquet.CompressionCodec_ZSTD
	}
	pw.CompressionType = compressionType

	// Extract API proxy from directory structure (app=<apiProxy>)
	dirPath := filepath.Dir(filename)
	// Get the parent directory which should be app=<apiProxy>
	appDir := filepath.Base(dirPath)
	apiProxy := ""

	// Check if the directory starts with "app="
	if len(appDir) > 4 && appDir[:4] == "app=" {
		apiProxy = appDir[4:] // Extract the apiProxy part
	} else {
		// Fall back to the old method if we can't find the app part
		apiProxy = filepath.Base(filename)
		apiProxy = apiProxy[:len(apiProxy)-len(filepath.Ext(apiProxy))] // Remove extension
	}

	// Write metrics to Parquet file
	for _, metric := range metrics {
		// Derive the date from each record's timestamp using RFC3339 date format
		// Use UTC to ensure consistent date regardless of local timezone
		recordDate := metric.Timestamp.UTC().Format(time.DateOnly)

		record := MetricRecord{
			Timestamp:  metric.Timestamp.UnixMilli(),
			MetricName: metric.Name,
			Value:      metric.Value,
			ApiProxy:   apiProxy,
			Labels:     metric.Labels,
			Date:       recordDate,
		}

		if err := pw.Write(record); err != nil {
			return fmt.Errorf("failed to write record: %w", err)
		}
	}

	// Finalize writing
	if err := pw.WriteStop(); err != nil {
		return fmt.Errorf("failed to finalize Parquet file: %w", err)
	}

	return nil
}
