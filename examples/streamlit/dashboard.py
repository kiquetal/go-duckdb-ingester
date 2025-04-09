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

# API Proxy Color Customization
st.sidebar.subheader("API Proxy Colors")
st.sidebar.markdown("Customize the colors for each API proxy:")

# Initialize color map in session state if not already set
if 'api_proxy_colors' not in st.session_state:
    st.session_state.api_proxy_colors = {}

# Default colors for API proxies (a list of distinct colors)
default_colors = [
    "#1f77b4", "#ff7f0e", "#2ca02c", "#d62728", "#9467bd",
    "#8c564b", "#e377c2", "#7f7f7f", "#bcbd22", "#17becf"
]

# Add color pickers for each selected API proxy
for i, api_proxy in enumerate(selected_api_proxies):
    # Use default color if not already set
    if api_proxy not in st.session_state.api_proxy_colors:
        st.session_state.api_proxy_colors[api_proxy] = default_colors[i % len(default_colors)]

    # Add color picker
    color = st.sidebar.color_picker(
        f"{api_proxy}",
        value=st.session_state.api_proxy_colors[api_proxy]
    )

    # Update color in session state
    st.session_state.api_proxy_colors[api_proxy] = color

# Create color mapping dictionary for Plotly
color_map = {api_proxy: st.session_state.api_proxy_colors[api_proxy]
             for api_proxy in selected_api_proxies}

# Metric selection
metrics = ["request_count"]
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

# Convert labels to readable format
if 'labels' in data.columns:
    data['labels'] = data['labels'].apply(lambda x: str(x) if x else '')

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
                color_discrete_map=color_map,
                title="Request Count Over Time",
                labels={"value": "Request Count", "timestamp": "Time", "api_proxy": "API Proxy"}
            )
            st.plotly_chart(fig, use_container_width=True)
        else:
            st.info("No request count data available for the selected filters.")

with tab2:
    st.subheader("Aggregated Metrics")

    # Connect to DuckDB for aggregations
    conn = duckdb.connect(database=':memory:')

    # Register the DataFrame as a table
    conn.register("metrics_data", data)

    # Total requests by API proxy and date
    if "request_count" in selected_metrics:
        st.write("### Total Requests by API Proxy and Date")
        total_requests = conn.execute("""
        SELECT 
            date,
            api_proxy,
            SUM(value) as total_requests
        FROM metrics_data
        WHERE metric_name = 'request_count'
        GROUP BY date, api_proxy
        ORDER BY date, total_requests DESC
        """).fetchdf()

        if len(total_requests) > 0:
            fig = px.bar(
                total_requests,
                x="date",
                y="total_requests",
                color="api_proxy",
                color_discrete_map=color_map,
                barmode="group",
                title="Total Requests by API Proxy and Date",
                labels={"total_requests": "Total Requests", "api_proxy": "API Proxy", "date": "Date"}
            )
            st.plotly_chart(fig, use_container_width=True)
            st.dataframe(total_requests)
        else:
            st.info("No request count data available for the selected filters.")

with tab3:
    st.subheader("Raw Data")

    # Initialize session state for pagination if not already set
    if 'page_size' not in st.session_state:
        st.session_state.page_size = 25
    if 'current_page' not in st.session_state:
        st.session_state.current_page = 1

    # Add pagination controls
    col1, col2 = st.columns([1, 3])
    with col1:
        page_size = st.selectbox(
            "Rows per page",
            options=[10, 25, 50, 100],
            index=[10, 25, 50, 100].index(st.session_state.page_size)
        )
        # Update session state if page size changes
        if page_size != st.session_state.page_size:
            st.session_state.page_size = page_size
            st.session_state.current_page = 1  # Reset to first page when changing page size

    # Calculate total pages
    total_rows = len(data)
    total_pages = max(1, (total_rows - 1) // page_size + 1)

    # Ensure current page is valid
    if st.session_state.current_page > total_pages:
        st.session_state.current_page = total_pages

    # Add page navigation
    col1, col2, col3 = st.columns([1, 3, 1])
    with col1:
        current_page = st.number_input(
            "Page",
            min_value=1,
            max_value=total_pages,
            value=st.session_state.current_page,
            step=1
        )
        # Update session state if page number changes
        if current_page != st.session_state.current_page:
            st.session_state.current_page = current_page
    with col3:
        st.write(f"Total: {total_rows} rows, {total_pages} pages")

    # Calculate start and end indices
    start_idx = (st.session_state.current_page - 1) * page_size
    end_idx = min(start_idx + page_size, total_rows)

    # Display current page of data
    st.dataframe(data.iloc[start_idx:end_idx])

    # Add page navigation buttons
    col1, col2, col3, col4 = st.columns([1, 1, 1, 1])
    with col1:
        if st.session_state.current_page > 1:
            if st.button("⏮️ First"):
                st.session_state.current_page = 1
                st.rerun()
    with col2:
        if st.session_state.current_page > 1:
            if st.button("◀️ Previous"):
                st.session_state.current_page -= 1
                st.rerun()
    with col3:
        if st.session_state.current_page < total_pages:
            if st.button("Next ▶️"):
                st.session_state.current_page += 1
                st.rerun()
    with col4:
        if st.session_state.current_page < total_pages:
            if st.button("Last ⏭️"):
                st.session_state.current_page = total_pages
                st.rerun()

# Footer
st.markdown("---")
st.markdown("Dashboard created with Streamlit, DuckDB, and Plotly.")
