import pandas as pd
import os
# Define the base data directory
data_directory = "../data"

# Dictionary to store the DataFrames
dataframes = {}

# Traverse the directory structure
for year_dir in os.listdir(data_directory):
    if year_dir.startswith("year="):
        year = year_dir.split("=")[1]
        year_path = os.path.join(data_directory, year_dir)
        for month_dir in os.listdir(year_path):
            if month_dir.startswith("month="):
                month = month_dir.split("=")[1]
                month_path = os.path.join(year_path, month_dir)
                for day_dir in os.listdir(month_path):
                    if day_dir.startswith("day="):
                        day = day_dir.split("=")[1]
                        day_path = os.path.join(month_path, day_dir)
                        for app_dir in os.listdir(day_path):
                            if app_dir.startswith("app="):
                                app = app_dir.split("=")[1]
                                app_path = os.path.join(day_path, app_dir)
                                # Look for all parquet files in the app directory
                                parquet_files = [f for f in os.listdir(app_path) if f.endswith('.parquet')]

                                if not parquet_files:
                                    print(f"Warning: No parquet files found in {app_path}")
                                    continue

                                # Process each parquet file
                                for parquet_filename in parquet_files:
                                    parquet_file = os.path.join(app_path, parquet_filename)
                                    try:
                                        df = pd.read_parquet(parquet_file)
                                        # Use a key that includes the specific file for uniqueness
                                        key = f"{year}-{month}-{day}-{app}-{parquet_filename}"
                                        dataframes[key] = df
                                        print(f"Successfully read: {parquet_file} (Key: {key})")
                                        print(df)
                                        print("-" * 50)
                                    except Exception as e:
                                        print(f"Error reading {parquet_file}: {e}")

# You can now access the DataFrames using the keys in the 'dataframes' dictionary
# For example:
# memento_metrics_df = dataframes.get("2025-04-07-memento-metrics_000000_060000.parquet")
#
# # Or you can access all DataFrames for a specific app by filtering keys
# memento_dfs = {k: v for k, v in dataframes.items() if k.startswith("2025-04-07-memento")}

# If you want to combine the data from all files (assuming compatible structures):
if dataframes:
    combined_df = pd.concat(dataframes.values(), ignore_index=True)
    print("\nCombined DataFrame (first 5 rows of all read data):")
    print(combined_df.head())
