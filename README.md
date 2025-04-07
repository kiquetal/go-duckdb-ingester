# Prometheus Metrics Collector with DuckDB and Streamlit Integration

This project provides a solution for collecting Prometheus metrics for specific API proxies, storing them in Parquet files, and analyzing them using DuckDB and Streamlit.

## Features

- Daily collection of Prometheus metrics for specified API proxies
- Storage of metrics in Parquet format with daily partitioning
- Query and analyze metrics using DuckDB
- Visualize metrics with Streamlit dashboards

## Architecture

The solution consists of three main components:

1. **Metrics Collector**: A Go application that queries Prometheus for metrics and stores them in Parquet files
2. **DuckDB Integration**: Python scripts to query and analyze the Parquet files using DuckDB
3. **Streamlit Dashboard**: Interactive web dashboard to visualize the metrics

## Prerequisites

- Go 1.22 or later
- Python 3.8 or later
- Prometheus server with API proxy metrics

## Installation

### Go Application

1. Clone the repository:
   ```bash
   git clone https://github.com/kiquetal/go-duckdb-ingester.git
   cd go-duckdb-ingester
   ```

2. Install Go dependencies:
   ```bash
   go mod tidy
   ```

3. Build the application:
   ```bash
   go build -o metrics-collector ./cmd/ingester
   ```

### Python Dependencies

Install the required Python packages for DuckDB and Streamlit:

```bash
# Install from requirements.txt
pip install -r examples/requirements.txt

# Or install packages individually
pip install duckdb pandas streamlit plotly
```

## Configuration

The application is configured using a YAML file. A sample configuration is provided in `config/config.yaml`.

### Configuration Options

```yaml
# Debug mode enables more verbose logging and shorter collection intervals
debug: false

# List of API proxies to collect metrics for
apiProxies:
  - "api-proxy-1"
  - "api-proxy-2"

# Prometheus connection settings
prometheus:
  # Prometheus server URL
  url: "http://prometheus:9090"

  # Timeout for Prometheus API requests
  timeout: 30s

  # Optional basic auth credentials
  # username: "prometheus"
  # password: "secret"

  # Metrics to collect
  metrics:
    - name: "request_count"
      # Use %s as a placeholder for the API proxy name
      query: 'sum(increase(apigee_request_count{apiproxy="%s"}[1d])) by (apiproxy, environment)'
      labels:
        - "apiproxy"
        - "environment"

# Storage configuration
storage:
  # Directory where Parquet files will be stored
  outputDir: "./data"

  # Compression algorithm (snappy, gzip, lz4, zstd)
  compression: "snappy"

  # Row group size in bytes (default: 128MB)
  rowGroupSize: 134217728
```

### Key Configuration Points

1. **API Proxies**: List the specific API proxies you want to collect metrics for
2. **Prometheus Metrics**: Define the metrics to collect with their PromQL queries
3. **Storage**: Configure where and how to store the Parquet files

## Usage

### Running the Metrics Collector

Run the metrics collector with a specific configuration file:

```bash
./metrics-collector --config config/config.yaml
```

The collector will:
1. Connect to Prometheus
2. Query metrics for each specified API proxy
3. Store the results in Parquet files with daily partitioning
4. By default, collect metrics every 24 hours

The Parquet files will be stored in the configured output directory with the following structure:

```
data/
  ├── 2023-04-01/
  │   ├── api-proxy-1.parquet
  │   ├── api-proxy-2.parquet
  │   └── api-proxy-3.parquet
  ├── 2023-04-02/
  │   ├── api-proxy-1.parquet
  │   ├── api-proxy-2.parquet
  │   └── api-proxy-3.parquet
  └── ...
```

### Querying Metrics with DuckDB

The repository includes an example script to query the Parquet files using DuckDB:

```bash
cd examples/duckdb
python query_metrics.py --data-dir ../../data --last-days 7
```

Options:
- `--data-dir`: Directory containing the Parquet files
- `--date`: Specific date to query (YYYY-MM-DD)
- `--last-days`: Query data from the last N days
- `--api-proxy`: Filter by specific API proxy
- `--metric`: Filter by specific metric name
- `--output`: Save results to CSV file

### DuckDB Query Examples

Here are some example DuckDB queries you can use to analyze the metrics:

#### Basic Query

```sql
SELECT 
    TIMESTAMP_MS(timestamp) as timestamp,
    metric_name,
    value,
    api_proxy,
    date
FROM 'data/2023-04-01/api-proxy-1.parquet'
ORDER BY timestamp;
```

#### Aggregation by API Proxy

```sql
SELECT 
    api_proxy,
    SUM(value) as total_requests
FROM 'data/2023-04-01/*.parquet'
WHERE metric_name = 'request_count'
GROUP BY api_proxy
ORDER BY total_requests DESC;
```

#### Time Series Analysis

```sql
SELECT 
    date,
    api_proxy,
    SUM(value) as daily_requests
FROM 'data/*/api-proxy-*.parquet'
WHERE metric_name = 'request_count'
GROUP BY date, api_proxy
ORDER BY date, api_proxy;
```

### Visualizing Metrics with Streamlit

The repository includes a Streamlit dashboard to visualize the metrics:

```bash
cd examples/streamlit
streamlit run dashboard.py
```

The dashboard provides:
- Interactive filters for date range, API proxies, and metrics
- Time series visualizations
- Aggregated metrics by API proxy
- Raw data view

## Extending the Solution

### Adding New Metrics

To add new metrics:

1. Update the configuration file with the new metric definition:
   ```yaml
   metrics:
     - name: "new_metric"
       query: 'your_promql_query{apiproxy="%s"}'
       labels:
         - "label1"
         - "label2"
   ```

2. Restart the metrics collector

### Custom Dashboards

You can create custom Streamlit dashboards by:

1. Copying and modifying the example dashboard
2. Adding new visualizations using Plotly
3. Creating custom DuckDB queries for specific analysis needs

## Troubleshooting

### Common Issues

1. **No data collected**:
   - Check Prometheus connection settings
   - Verify API proxy names are correct
   - Ensure PromQL queries are valid

2. **DuckDB query errors**:
   - Verify Parquet files exist in the expected location
   - Check file permissions
   - Ensure DuckDB and dependencies are installed correctly

3. **Streamlit dashboard errors**:
   - Check Python dependencies are installed
   - Verify data directory path is correct

## License

This project is licensed under the MIT License - see the LICENSE file for details.
