#!/usr/bin/env python3
"""
Example Streamlit dashboard for visualizing Prometheus metrics stored in Parquet files.
"""

import os
import sys
import streamlit as st
import duckdb
import pandas as pd
import plotly.express as px
import plotly.graph_objects as go
from datetime import datetime, timedelta

# Check if the script is being run directly with Python instead of with Streamlit
if __name__ == "__main__":
    # When running with `streamlit run`, streamlit sets up the environment differently
    # We can detect this by checking if we can access streamlit's session state
    try:
        # Try to access a Streamlit-specific function that would fail if not running in Streamlit
        # This will raise an exception if not running in Streamlit
        _ = st.session_state
    except Exception:
        print("Error: This is a Streamlit app and should not be run directly with Python.")
        print("Please use the following command to run the app:")
        print(f"    streamlit run {os.path.basename(__file__)} [ARGUMENTS]")
        sys.exit(1)

# Set page configuration
st.set_page_config(
    page_title="API Metrics Dashboard",
    page_icon="📊",
    layout="wide",
    initial_sidebar_state="expanded",
)

# Title and description
st.title("API Metrics Dashboard")
st.markdown("""
This dashboard visualizes API metrics collected from Prometheus and stored in Parquet files.
Use the sidebar to filter the data and explore different metrics.
""")

# Sidebar filters
st.sidebar.header("Filters")

# Function to get available dates from partitioned structure
@st.cache_data
def get_available_dates(data_dir):
    dates = []
    if os.path.exists(data_dir):
        # Check for partitioned structure: year=YYYY/month=MM/day=DD
        for year_dir in os.listdir(data_dir):
            if year_dir.startswith("year="):
                year = year_dir.split("=")[1]
                year_path = os.path.join(data_dir, year_dir)

                for month_dir in os.listdir(year_path):
                    if month_dir.startswith("month="):
                        month = month_dir.split("=")[1]
                        month_path = os.path.join(year_path, month_dir)

                        for day_dir in os.listdir(month_path):
                            if day_dir.startswith("day="):
                                day = day_dir.split("=")[1]
                                # Format as YYYY-MM-DD
                                date_str = f"{year}-{month}-{day}"
                                dates.append(date_str)
    return sorted(dates, reverse=True)

# Function to get available API proxies from partitioned structure
@st.cache_data
def get_available_api_proxies(data_dir, selected_dates):
    api_proxies = set()
    for date in selected_dates:
        # Parse the date to get year, month, day
        year, month, day = date.split('-')

        # Construct the path to the day directory
        day_path = os.path.join(data_dir, f"year={year}", f"month={month}", f"day={day}")

        if os.path.exists(day_path):
            for app_dir in os.listdir(day_path):
                if app_dir.startswith("app="):
                    # Extract the app name from the directory name
                    app_name = app_dir.split("=")[1]
                    api_proxies.add(app_name)

    return sorted(list(api_proxies))

# Data directory
data_dir = st.sidebar.text_input("Data Directory", value="../../data")

# Get available dates
available_dates = get_available_dates(data_dir)
if not available_dates:
    st.error(f"No data found in {data_dir}. Please check the directory path.")
    st.stop()

# Date range selection
date_range = st.sidebar.date_input(
    "Date Range",
    value=(
        datetime.strptime(available_dates[-1], "%Y-%m-%d").date(),
        datetime.strptime(available_dates[0], "%Y-%m-%d").date()
    ),
    min_value=datetime.strptime(available_dates[-1], "%Y-%m-%d").date(),
    max_value=datetime.strptime(available_dates[0], "%Y-%m-%d").date(),
)

# Convert date range to list of date strings
if isinstance(date_range, tuple) and len(date_range) == 2:
    start_date, end_date = date_range
    selected_dates = [
        (start_date + timedelta(days=i)).strftime("%Y-%m-%d")
        for i in range((end_date - start_date).days + 1)
    ]
else:
    selected_dates = [date_range.strftime("%Y-%m-%d")]

# Filter to dates that actually have data
selected_dates = [date for date in selected_dates if date in available_dates]

# Get available API proxies for selected dates
available_api_proxies = get_available_api_proxies(data_dir, selected_dates)
if not available_api_proxies:
    st.error(f"No API proxies found for the selected date range.")
    st.stop()

# API proxy selection
selected_api_proxies = st.sidebar.multiselect(
    "API Proxies",
    options=available_api_proxies,
    default=available_api_proxies[:3] if len(available_api_proxies) > 3 else available_api_proxies
)

if not selected_api_proxies:
    st.warning("Please select at least one API proxy.")
    st.stop()

# Metric selection
metrics = ["request_count", "response_time", "error_count"]
selected_metrics = st.sidebar.multiselect(
    "Metrics",
    options=metrics,
    default=metrics
)

if not selected_metrics:
    st.warning("Please select at least one metric.")
    st.stop()

# Function to load data from partitioned structure
@st.cache_data
def load_data(data_dir, selected_dates, selected_api_proxies, selected_metrics):
    # Create a list of all Parquet files to query
    parquet_files = []
    for date in selected_dates:
        # Parse the date to get year, month, day
        year, month, day = date.split('-')

        # Construct the path to the day directory
        day_path = os.path.join(data_dir, f"year={year}", f"month={month}", f"day={day}")

        if os.path.exists(day_path):
            for api_proxy in selected_api_proxies:
                # Construct the path to the app directory
                app_path = os.path.join(day_path, f"app={api_proxy}")

                if os.path.exists(app_path):
                    # Find all parquet files in the app directory
                    for file in os.listdir(app_path):
                        if file.endswith('.parquet'):
                            file_path = os.path.join(app_path, file)
                            parquet_files.append(file_path)

    if not parquet_files:
        return None

    # Connect to DuckDB
    conn = duckdb.connect(database=':memory:')

    # Build query
    metrics_clause = ", ".join([f"'{m}'" for m in selected_metrics])

    query = f"""
SELECT
    timestamp AS timestamp,
    metric_name,
    value,
    api_proxy,
    date,
    labels
FROM parquet_scan([{''.join(f"'{f}', " for f in parquet_files)[:-2]}])
WHERE metric_name IN ({metrics_clause})
ORDER BY timestamp
"""

    try:
        # Execute query
        result = conn.execute(query).fetchdf()
        return result
    except Exception as e:
        st.error(f"Error loading data: {e}")
        return None

# Load data
with st.spinner("Loading data..."):
    data = load_data(data_dir, selected_dates, selected_api_proxies, selected_metrics)

if data is None or len(data) == 0:
    st.error("No data found for the selected filters.")
    st.stop()

# Display data overview
st.subheader("Data Overview")
st.write(f"Loaded {len(data)} records from {len(selected_dates)} days for {len(selected_api_proxies)} API proxies.")

# Create tabs for different visualizations
tab1, tab2, tab3 = st.tabs(["Time Series", "Aggregations", "Raw Data"])

with tab1:
    st.subheader("Time Series Visualization")

    # Group data by timestamp and API proxy
    if "request_count" in selected_metrics:
        st.write("### Request Count Over Time")
        request_data = data[data["metric_name"] == "request_count"]
        if len(request_data) > 0:
            fig = px.line(
                request_data,
                x="timestamp",
                y="value",
                color="api_proxy",
                title="Request Count Over Time",
                labels={"value": "Request Count", "timestamp": "Time", "api_proxy": "API Proxy"}
            )
            st.plotly_chart(fig, use_container_width=True)
        else:
            st.info("No request count data available for the selected filters.")

    if "response_time" in selected_metrics:
        st.write("### Response Time Over Time")
        response_data = data[data["metric_name"] == "response_time"]
        if len(response_data) > 0:
            fig = px.line(
                response_data,
                x="timestamp",
                y="value",
                color="api_proxy",
                title="Response Time Over Time",
                labels={"value": "Response Time (ms)", "timestamp": "Time", "api_proxy": "API Proxy"}
            )
            st.plotly_chart(fig, use_container_width=True)
        else:
            st.info("No response time data available for the selected filters.")

    if "error_count" in selected_metrics:
        st.write("### Error Count Over Time")
        error_data = data[data["metric_name"] == "error_count"]
        if len(error_data) > 0:
            fig = px.line(
                error_data,
                x="timestamp",
                y="value",
                color="api_proxy",
                title="Error Count Over Time",
                labels={"value": "Error Count", "timestamp": "Time", "api_proxy": "API Proxy"}
            )
            st.plotly_chart(fig, use_container_width=True)
        else:
            st.info("No error count data available for the selected filters.")

with tab2:
    st.subheader("Aggregated Metrics")

    # Connect to DuckDB for aggregations
    conn = duckdb.connect(database=':memory:')

    # Register the DataFrame as a table
    conn.register("metrics_data", data)

    # Total requests by API proxy
    if "request_count" in selected_metrics:
        st.write("### Total Requests by API Proxy")
        total_requests = conn.execute("""
        SELECT 
            api_proxy,
            SUM(value) as total_requests
        FROM metrics_data
        WHERE metric_name = 'request_count'
        GROUP BY api_proxy
        ORDER BY total_requests DESC
        """).fetchdf()

        if len(total_requests) > 0:
            fig = px.bar(
                total_requests,
                x="api_proxy",
                y="total_requests",
                title="Total Requests by API Proxy",
                labels={"total_requests": "Total Requests", "api_proxy": "API Proxy"}
            )
            st.plotly_chart(fig, use_container_width=True)
            st.dataframe(total_requests)
        else:
            st.info("No request count data available for the selected filters.")

    # Average response time by API proxy
    if "response_time" in selected_metrics:
        st.write("### Average Response Time by API Proxy")
        avg_response_time = conn.execute("""
        SELECT 
            api_proxy,
            AVG(value) as avg_response_time_ms
        FROM metrics_data
        WHERE metric_name = 'response_time'
        GROUP BY api_proxy
        ORDER BY avg_response_time_ms DESC
        """).fetchdf()

        if len(avg_response_time) > 0:
            fig = px.bar(
                avg_response_time,
                x="api_proxy",
                y="avg_response_time_ms",
                title="Average Response Time by API Proxy",
                labels={"avg_response_time_ms": "Avg Response Time (ms)", "api_proxy": "API Proxy"}
            )
            st.plotly_chart(fig, use_container_width=True)
            st.dataframe(avg_response_time)
        else:
            st.info("No response time data available for the selected filters.")

    # Error count by API proxy
    if "error_count" in selected_metrics:
        st.write("### Error Count by API Proxy")
        error_counts = conn.execute("""
        SELECT 
            api_proxy,
            SUM(value) as error_count
        FROM metrics_data
        WHERE metric_name = 'error_count'
        GROUP BY api_proxy
        ORDER BY error_count DESC
        """).fetchdf()

        if len(error_counts) > 0:
            fig = px.bar(
                error_counts,
                x="api_proxy",
                y="error_count",
                title="Error Count by API Proxy",
                labels={"error_count": "Error Count", "api_proxy": "API Proxy"}
            )
            st.plotly_chart(fig, use_container_width=True)
            st.dataframe(error_counts)
        else:
            st.info("No error count data available for the selected filters.")

with tab3:
    st.subheader("Raw Data")
    st.dataframe(data)

# Footer
st.markdown("---")
st.markdown("Dashboard created with Streamlit, DuckDB, and Plotly.")
