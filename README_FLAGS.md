# Command Line Flags for the Prometheus Metrics Collector

This document explains how to use the command line flags available in the Prometheus Metrics Collector application.

## Available Flags

Currently, the application supports the following command line flags:

### `--config` Flag

This flag allows you to specify the path to the configuration file.

**Default value:** `config.yaml` (in the current directory)

**Usage examples:**

```bash
# Using the default configuration file (config.yaml in the current directory)
./metrics-collector

# Specifying a different configuration file
./metrics-collector --config=custom-config.yaml

# Alternative syntax
./metrics-collector -config custom-config.yaml

# Using a configuration file in a different directory
./metrics-collector --config=/path/to/my/config.yaml
```

### `--start` Flag

This flag allows you to specify the start time for a range query in RFC3339 format.

**Default value:** None (current time is used if not specified)

**Usage examples:**

```bash
# Specify a start time for the range query
./metrics-collector --start="2025-04-07T00:00:00Z"
```

### `--end` Flag

This flag allows you to specify the end time for a range query in RFC3339 format.

**Default value:** None (current time is used if not specified)

**Usage examples:**

```bash
# Specify an end time for the range query
./metrics-collector --end="2025-04-08T00:00:00Z"
```

### `--range` Flag

This flag enables range queries instead of instant queries. When used with `--start` and `--end` flags, it collects metrics over the specified time range with the step interval configured in the configuration file.

**Default value:** `false`

**Usage examples:**

```bash
# Enable range queries
./metrics-collector --range

# Enable range queries with specific start and end times
./metrics-collector --range --start="2025-04-07T00:00:00Z" --end="2025-04-08T00:00:00Z"
```

## Memory Usage Optimization

When using range queries with `--start` and `--end` flags for large time ranges (e.g., querying data for an entire day or more), the application automatically processes data in batches to reduce memory consumption. This is especially important when dealing with historical data.

The application:
1. Divides the specified time range into smaller batches (6-hour chunks by default)
2. Processes each batch sequentially
3. Creates separate Parquet files for each batch with timestamps in the filename
4. Performs garbage collection between batches to free up memory

This approach significantly reduces memory usage compared to processing the entire time range at once.

### Example Usage for Historical Data

```bash
# Query metrics for a full day with optimized memory usage
./metrics-collector --range --start="2025-04-07T00:00:00Z" --end="2025-04-08T00:00:00Z"
```

The above command will create multiple Parquet files with names like:
- `metrics_000000_060000.parquet` (data from 00:00:00 to 06:00:00)
- `metrics_060000_120000.parquet` (data from 06:00:00 to 12:00:00)
- `metrics_120000_180000.parquet` (data from 12:00:00 to 18:00:00)
- `metrics_180000_240000.parquet` (data from 18:00:00 to 24:00:00)

## How It Works

In the application code, the flag is defined and parsed as follows:

```
// Parse command line flags
configPath := flag.String("config", "config.yaml", "Path to configuration file")
flag.Parse()
```

This creates a string flag named "config" with:
- Default value: "config.yaml"
- Description: "Path to configuration file"

After parsing, the flag value is available as `*configPath` (note the dereference operator `*` since `configPath` is a pointer).

## Adding New Flags

To add more command line flags to the application, you would:

1. Define them using the appropriate flag type function (`flag.String()`, `flag.Int()`, `flag.Bool()`, etc.)
2. Access their values after calling `flag.Parse()`

For example, to add a debug flag:

```
debugMode := flag.Bool("debug", false, "Enable debug mode")
flag.Parse()

if *debugMode {
    // Enable debug features
}
```
