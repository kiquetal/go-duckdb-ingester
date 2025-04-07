#!/usr/bin/env python3
"""
Example script to query Prometheus metrics stored in Parquet files using DuckDB.
"""

import os
import sys
import argparse
import duckdb
import pandas as pd
from datetime import datetime, timedelta

def parse_args():
    parser = argparse.ArgumentParser(description='Query Prometheus metrics from Parquet files using DuckDB')
    parser.add_argument('--data-dir', default='../../data', help='Directory containing Parquet files')
    parser.add_argument('--date', help='Date to query (YYYY-MM-DD), defaults to today')
    parser.add_argument('--api-proxy', help='Filter by specific API proxy')
    parser.add_argument('--metric', help='Filter by specific metric name')
    parser.add_argument('--output', help='Output file (CSV format)')
    parser.add_argument('--last-days', type=int, help='Query data from the last N days')
    return parser.parse_args()

def main():
    args = parse_args()
    
    # Connect to DuckDB (in-memory database)
    conn = duckdb.connect(database=':memory:')
    
    # Determine date range
    if args.last_days:
        end_date = datetime.now()
        start_date = end_date - timedelta(days=args.last_days)
        date_range = [(start_date + timedelta(days=i)).strftime('%Y-%m-%d') 
                      for i in range((end_date - start_date).days + 1)]
    elif args.date:
        date_range = [args.date]
    else:
        date_range = [datetime.now().strftime('%Y-%m-%d')]
    
    # Build query conditions
    conditions = []
    if args.api_proxy:
        conditions.append(f"api_proxy = '{args.api_proxy}'")
    if args.metric:
        conditions.append(f"metric_name = '{args.metric}'")
    
    where_clause = " AND ".join(conditions)
    if where_clause:
        where_clause = f"WHERE {where_clause}"
    
    # Create a list of all Parquet files to query
    parquet_files = []
    for date in date_range:
        date_dir = os.path.join(args.data_dir, date)
        if os.path.exists(date_dir):
            for file in os.listdir(date_dir):
                if file.endswith('.parquet'):
                    parquet_files.append(os.path.join(date_dir, file))
    
    if not parquet_files:
        print(f"No Parquet files found in {args.data_dir} for the specified date range.")
        return 1
    
    # Query all Parquet files
    query = f"""
    SELECT 
        TIMESTAMP_MS(timestamp) as timestamp,
        metric_name,
        value,
        api_proxy,
        date,
        labels
    FROM parquet_scan([{''.join(f"'{f}', " for f in parquet_files)[:-2]}])
    {where_clause}
    ORDER BY timestamp
    """
    
    try:
        # Execute query
        result = conn.execute(query).fetchdf()
        
        # Display results
        if len(result) > 0:
            print(f"Found {len(result)} records.")
            print(result.head())
            
            # Save to CSV if requested
            if args.output:
                result.to_csv(args.output, index=False)
                print(f"Results saved to {args.output}")
        else:
            print("No data found matching the criteria.")
        
        # Example of aggregation queries
        print("\nExample aggregations:")
        
        # Total requests by API proxy
        if 'request_count' in result['metric_name'].values:
            total_requests = conn.execute(f"""
            SELECT 
                api_proxy,
                SUM(value) as total_requests
            FROM parquet_scan([{''.join(f"'{f}', " for f in parquet_files)[:-2]}])
            WHERE metric_name = 'request_count'
            GROUP BY api_proxy
            ORDER BY total_requests DESC
            """).fetchdf()
            
            print("\nTotal requests by API proxy:")
            print(total_requests)
        
        # Average response time by API proxy
        if 'response_time' in result['metric_name'].values:
            avg_response_time = conn.execute(f"""
            SELECT 
                api_proxy,
                AVG(value) as avg_response_time_ms
            FROM parquet_scan([{''.join(f"'{f}', " for f in parquet_files)[:-2]}])
            WHERE metric_name = 'response_time'
            GROUP BY api_proxy
            ORDER BY avg_response_time_ms DESC
            """).fetchdf()
            
            print("\nAverage response time by API proxy (ms):")
            print(avg_response_time)
        
        # Error count by API proxy and error type
        if 'error_count' in result['metric_name'].values:
            error_counts = conn.execute(f"""
            SELECT 
                api_proxy,
                labels['error_type'] as error_type,
                SUM(value) as error_count
            FROM parquet_scan([{''.join(f"'{f}', " for f in parquet_files)[:-2]}])
            WHERE metric_name = 'error_count'
            GROUP BY api_proxy, labels['error_type']
            ORDER BY error_count DESC
            """).fetchdf()
            
            print("\nError counts by API proxy and error type:")
            print(error_counts)
        
    except Exception as e:
        print(f"Error executing query: {e}")
        return 1
    
    return 0

if __name__ == "__main__":
    sys.exit(main())
