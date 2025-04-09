package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/kiquetal/go-duckdb-ingester/internal/prometheus"
	"github.com/kiquetal/go-duckdb-ingester/internal/storage"
	"github.com/kiquetal/go-duckdb-ingester/pkg/config"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	startTimeStr := flag.String("start", "", "Start time for range query (RFC3339 format, e.g., 2025-04-07T00:00:00Z)")
	endTimeStr := flag.String("end", "", "End time for range query (RFC3339 format, e.g., 2025-04-08T00:00:00Z)")
	useRangeQuery := flag.Bool("range", false, "Use range query instead of instant query")
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Override configuration with command line flags if provided
	if *useRangeQuery {
		cfg.Prometheus.UseRangeQuery = true
	}

	// Parse start and end times if provided
	if *startTimeStr != "" && *endTimeStr != "" {
		startTime, err := time.Parse(time.RFC3339, *startTimeStr)
		if err != nil {
			log.Fatalf("Failed to parse start time: %v", err)
		}

		endTime, err := time.Parse(time.RFC3339, *endTimeStr)
		if err != nil {
			log.Fatalf("Failed to parse end time: %v", err)
		}

		// Store the time range in the configuration
		cfg.Prometheus.UseRangeQuery = true
		cfg.StartTime = startTime
		cfg.EndTime = endTime
	}

	// Initialize Prometheus client
	promClient, err := prometheus.NewClient(cfg.Prometheus)
	if err != nil {
		log.Fatalf("Failed to create Prometheus client: %v", err)
	}

	// Initialize storage
	store, err := storage.NewParquetStorage(cfg.Storage)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}

	// Setup signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Create ticker for daily collection
	ticker := time.NewTicker(24 * time.Hour)
	if cfg.Debug {
		// Use shorter interval for debugging
		ticker = time.NewTicker(1 * time.Minute)
	}

	// Run initial collection
	collectAndStore(promClient, store, cfg)

	// Main loop
	fmt.Println("Starting metrics collection. Press Ctrl+C to exit.")
	for {
		select {
		case <-ticker.C:
			collectAndStore(promClient, store, cfg)
		case <-sigCh:
			fmt.Println("Shutting down...")
			ticker.Stop()
			return
		}
	}
}

func collectAndStore(client *prometheus.Client, store *storage.ParquetStorage, cfg *config.Config) {
	totalStartTime := time.Now()
	log.Printf("Collecting metrics for API proxies: %v", cfg.APIProxies)

	// Determine the date to use for file partitioning
	var fileDate time.Time
	if !cfg.StartTime.IsZero() {
		// If start time is provided, use it for file partitioning
		fileDate = cfg.StartTime
	} else {
		// Otherwise use current time
		fileDate = time.Now()
	}

	year := fileDate.Format("2006")
	month := fileDate.Format("01")
	day := fileDate.Format("02")

	// Process each API proxy sequentially to reduce memory usage
	for _, apiProxy := range cfg.APIProxies {
		if cfg.Prometheus.UseRangeQuery && !cfg.StartTime.IsZero() && !cfg.EndTime.IsZero() {
			// Use range query if enabled and start/end times are provided
			log.Printf("Processing metrics for %s using range query from %s to %s with step %s",
				apiProxy, cfg.StartTime.Format(time.RFC3339), cfg.EndTime.Format(time.RFC3339),
				cfg.Prometheus.RangeStep)

			// Calculate the total duration
			totalDuration := cfg.EndTime.Sub(cfg.StartTime)

			// Use a batch size of 6 hours to reduce memory usage
			batchDuration := 6 * time.Hour

			// If the total duration is less than the batch size, just use the total duration
			if totalDuration < batchDuration {
				batchDuration = totalDuration
			}

			// Process data in batches to reduce memory usage
			for batchStart := cfg.StartTime; batchStart.Before(cfg.EndTime); batchStart = batchStart.Add(batchDuration) {
				batchEnd := batchStart.Add(batchDuration)
				if batchEnd.After(cfg.EndTime) {
					batchEnd = cfg.EndTime
				}

				log.Printf("Collecting batch for %s from %s to %s",
					apiProxy, batchStart.Format(time.RFC3339), batchEnd.Format(time.RFC3339))

				timeRange := prometheus.TimeRange{
					Start: batchStart,
					End:   batchEnd,
					Step:  cfg.Prometheus.RangeStep,
				}

				// Measure time for Prometheus query
				queryStartTime := time.Now()
				metrics, err := client.CollectMetricsRange(apiProxy, timeRange)
				queryDuration := time.Since(queryStartTime)
				log.Printf("Prometheus range query for %s took %s", apiProxy, queryDuration)

				if err != nil {
					log.Printf("Error collecting metrics for %s: %v", apiProxy, err)
					continue
				}

				if len(metrics) == 0 {
					log.Printf("No metrics found for %s in this batch", apiProxy)
					continue
				}

				// Store metrics in parquet file with recommended partitioning structure
				// year=YYYY/month=MM/day=DD/app=apiProxy/metrics_HHMMSS_HHMMSS.parquet
				// Create a unique filename for each batch to avoid memory issues
				// Use the batch start time for file partitioning to ensure each day's data
				// is stored in the correct folder, especially when the query spans multiple days
				batchYear := batchStart.Format("2006")
				batchMonth := batchStart.Format("01")
				batchDay := batchStart.Format("02")

				batchFilename := fmt.Sprintf("%s/year=%s/month=%s/day=%s/app=%s/metrics_%s_%s.parquet",
					cfg.Storage.OutputDir, batchYear, batchMonth, batchDay, apiProxy,
					batchStart.Format("150405"), batchEnd.Format("150405"))

				// Measure time for Parquet file writing
				writeStartTime := time.Now()
				if err := store.StoreMetrics(metrics, batchFilename); err != nil {
					log.Printf("Error storing metrics for %s: %v", apiProxy, err)
					// Continue processing even if there's an error
					log.Printf("Continuing to next batch despite error...")
				} else {
					writeDuration := time.Since(writeStartTime)
					log.Printf("Successfully stored metrics for %s in %s (took %s)", apiProxy, batchFilename, writeDuration)
				}

				// Force garbage collection to free up memory
				metrics = nil
				runtime.GC()

				// Log the next batch start time to help with debugging
				nextBatchStart := batchStart.Add(batchDuration)
				if nextBatchStart.Before(cfg.EndTime) {
					log.Printf("Next batch will start at %s", nextBatchStart.Format(time.RFC3339))
				} else {
					log.Printf("All batches processed for %s", apiProxy)
				}
			}
		} else {
			// Use instant query
			log.Printf("Collecting metrics for %s using instant query", apiProxy)

			// Measure time for Prometheus query
			queryStartTime := time.Now()
			metrics, err := client.CollectMetrics(apiProxy)
			queryDuration := time.Since(queryStartTime)
			log.Printf("Prometheus instant query for %s took %s", apiProxy, queryDuration)

			if err != nil {
				log.Printf("Error collecting metrics for %s: %v", apiProxy, err)
				continue
			}

			// Store metrics in parquet file with recommended partitioning structure
			// year=YYYY/month=MM/day=DD/app=apiProxy/metrics.parquet
			filename := fmt.Sprintf("%s/year=%s/month=%s/day=%s/app=%s/metrics.parquet",
				cfg.Storage.OutputDir, year, month, day, apiProxy)

			// Measure time for Parquet file writing
			writeStartTime := time.Now()
			if err := store.StoreMetrics(metrics, filename); err != nil {
				log.Printf("Error storing metrics for %s: %v", apiProxy, err)
				// Continue processing even if there's an error
				log.Printf("Continuing to next API proxy despite error...")
			} else {
				writeDuration := time.Since(writeStartTime)
				log.Printf("Successfully stored metrics for %s in %s (took %s)", apiProxy, filename, writeDuration)
			}
		}
	}

	// Log total time taken for the entire collection and storage process
	totalDuration := time.Since(totalStartTime)
	log.Printf("Total time for collecting and storing metrics: %s", totalDuration)
}
