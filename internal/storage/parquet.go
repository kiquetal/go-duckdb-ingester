package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/kiquetal/go-duckdb-ingester/internal/prometheus"
	"github.com/kiquetal/go-duckdb-ingester/pkg/config"
	"github.com/xitongsys/parquet-go-source/local"
	"github.com/xitongsys/parquet-go/parquet"
	"github.com/xitongsys/parquet-go/writer"
)

type Label struct {
	Key   string `parquet:"name=key, type=BYTE_ARRAY, convertedtype=UTF8"`
	Value string `parquet:"name=value, type=BYTE_ARRAY, convertedtype=UTF8"`
}

type MetricRecord struct {
	Timestamp  int64   `parquet:"name=timestamp, type=INT64, convertedtype=TIMESTAMP_MILLIS"`
	MetricName string  `parquet:"name=metric_name, type=BYTE_ARRAY, convertedtype=UTF8"`
	Value      float64 `parquet:"name=value, type=DOUBLE"`
	ApiProxy   string  `parquet:"name=api_proxy, type=BYTE_ARRAY, convertedtype=UTF8"`
	Labels     []Label `parquet:"name=labels, type=LIST, convertedtype=LIST"`
	Date       string  `parquet:"name=date, type=BYTE_ARRAY, convertedtype=UTF8"`
}

type ParquetStorage struct {
	config config.StorageConfig
}

func NewParquetStorage(cfg config.StorageConfig) (*ParquetStorage, error) {
	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}
	return &ParquetStorage{config: cfg}, nil
}

func (s *ParquetStorage) StoreMetrics(metrics []prometheus.MetricResult, filename string) error {
	if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	fw, err := local.NewLocalFileWriter(filename)
	if err != nil {
		return fmt.Errorf("failed to create file writer: %w", err)
	}
	defer fw.Close()

	pw, err := writer.NewParquetWriter(fw, new(MetricRecord), 4)
	if err != nil {
		return fmt.Errorf("failed to create parquet writer: %w", err)
	}

	// Configure writer
	pw.RowGroupSize = 128 * 1024 * 1024
	pw.PageSize = 8 * 1024
	pw.CompressionType = parquet.CompressionCodec_SNAPPY

	// Batch processing
	batchSize := 1000
	for i := 0; i < len(metrics); i += batchSize {
		end := i + batchSize
		if end > len(metrics) {
			end = len(metrics)
		}

		for _, metric := range metrics[i:end] {
			// Extract API proxy from labels if available
			apiProxy := ""
			if val, ok := metric.Labels["apiproxy"]; ok {
				apiProxy = val
			} else if val, ok := metric.Labels["app"]; ok { // Fallback to "app" label
				apiProxy = val
			}

			record := MetricRecord{
				Timestamp:  metric.Timestamp.UnixMilli(),
				MetricName: metric.Name,
				Value:      metric.Value,
				ApiProxy:   apiProxy,
				Labels:     convertLabels(metric.Labels),
				Date:       metric.Timestamp.UTC().Format(time.DateOnly),
			}
			if err := pw.Write(record); err != nil {
				return fmt.Errorf("write error: %w", err)
			}
		}
	}

	// Finalization with timeout
	done := make(chan struct{})
	var writeStopErr error
	go func() {
		defer close(done)
		writeStopErr = pw.WriteStop()
	}()

	select {
	case <-done:
		return writeStopErr
	case <-time.After(s.config.WriteStopTimeout):
		return fmt.Errorf("parquet finalization timed out after %s", s.config.WriteStopTimeout)
	}
}

func convertLabels(labels map[string]string) []Label {
	result := make([]Label, 0, len(labels))
	for k, v := range labels {
		result = append(result, Label{Key: k, Value: v})
	}
	return result
}
