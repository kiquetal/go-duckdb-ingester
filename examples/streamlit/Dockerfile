# Use Python 3.9 as the base image
FROM python:3.9-slim

# Set working directory
WORKDIR /app

# Copy the dashboard application
COPY dashboard.py .

# Install required dependencies
RUN pip install --no-cache-dir streamlit duckdb pandas plotly

# Expose the Streamlit port
EXPOSE 8501

# Set environment variables
ENV PYTHONUNBUFFERED=1

# Volume configuration
# Mount your data directory to /app/data when running the container
# Example: docker run -v /path/to/your/data:/app/data -p 8501:8501 streamlit-dashboard
# 
# The dashboard expects data in the following structure:
# /app/data/year=YYYY/month=MM/day=DD/app=NAME/*.parquet
#
# Default data directory in the app is "../../data" which maps to "/app/data" in the container

# Command to run the Streamlit app
# The --server.address=0.0.0.0 flag makes the server accessible from outside the container
CMD ["streamlit", "run", "dashboard.py", "--server.address=0.0.0.0", "--server.port=8501", "--", "--data_dir=/app/data"]