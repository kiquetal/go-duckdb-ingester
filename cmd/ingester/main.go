package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kiquetal/go-duckdb-ingester/internal/prometheus"
	"github.com/kiquetal/go-duckdb-ingester/internal/storage"
	"github.com/kiquetal/go-duckdb-ingester/pkg/config"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
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
	log.Printf("Collecting metrics for API proxies: %v", cfg.APIProxies)

	// Get current date for file partitioning
	currentDate := time.Now().Format("2006-01-02")

	for _, apiProxy := range cfg.APIProxies {
		// Collect metrics for the specific API proxy
		metrics, err := client.CollectMetrics(apiProxy)
		if err != nil {
			log.Printf("Error collecting metrics for %s: %v", apiProxy, err)
			continue
		}

		// Store metrics in parquet file with date partition
		filename := fmt.Sprintf("%s/%s/%s.parquet", cfg.Storage.OutputDir, currentDate, apiProxy)
		if err := store.StoreMetrics(metrics, filename); err != nil {
			log.Printf("Error storing metrics for %s: %v", apiProxy, err)
		} else {
			log.Printf("Successfully stored metrics for %s in %s", apiProxy, filename)
		}
	}
}
