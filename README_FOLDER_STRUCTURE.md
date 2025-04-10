# Parquet Folder Structure Guide

## Understanding Folder Structure

The metrics-collector creates Parquet files in a directory structure based on the start date provided in the command-line arguments. The structure follows this pattern:

```
{outputDir}/year={YYYY}/month={MM}/day={DD}/app={apiProxy}/metrics.parquet
```

For range queries, the filename includes timestamps:

```
{outputDir}/year={YYYY}/month={MM}/day={DD}/app={apiProxy}/metrics_{HHMMSS}_{HHMMSS}.parquet
```

## How Dates Are Used for Folder Creation

When you run the metrics-collector with `--start` and `--end` parameters, the application uses **only the start date** to determine the folder structure. This means that even if your query spans multiple days, all Parquet files will be stored in folders corresponding to the start date.

### Modifying the Code to Create Folders for Each Day in a Range

If you want to create folders for each day within a date range (instead of using only the start date), you need to modify the code in `cmd/ingester/main.go`. Here's how:

1. Locate the `collectAndStore` function (around line 95)
2. Find the section where the file date is determined (around line 100-107)
3. Keep the original code for non-batch processing:

```
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
```

4. Then, inside the batch processing loop (around line 133), add this code before creating the filename:

```
// Use the batch start time for file partitioning
batchYear := batchStart.Format("2006")
batchMonth := batchStart.Format("01")
batchDay := batchStart.Format("02")
```

5. Finally, modify the filename creation (around line 167) to use the batch date variables:

```
batchFilename := fmt.Sprintf("%s/year=%s/month=%s/day=%s/app=%s/metrics_%s_%s.parquet",
    cfg.Storage.OutputDir, batchYear, batchMonth, batchDay, apiProxy,
    batchStart.Format("150405"), batchEnd.Format("150405"))
```

With these changes, the application will create folders based on the actual date of each batch, rather than using only the start date for all batches.

## Creating Folders for a Specific Day

To create Parquet files in folders for a specific day (e.g., day 07), you need to set that day as the start date in your command.

### Example: Creating Folders for April 7, 2025

To create Parquet files in folders for April 7, 2025, run:

```bash
./metrics-collector --config config/config.yaml --start="2025-04-07T00:00:00Z" --end="2025-04-07T23:59:59Z"
```

This will create folders with the following structure:

```
./data/year=2025/month=04/day=07/app={apiProxy}/metrics_{HHMMSS}_{HHMMSS}.parquet
```

### Example: Creating Folders for Multiple Days

If you need to create folders for multiple days, you'll need to run the command multiple times, once for each day:

#### For April 6, 2025:
```bash
./metrics-collector --config config/config.yaml --start="2025-04-06T00:00:00Z" --end="2025-04-06T23:59:59Z"
```

#### For April 7, 2025:
```bash
./metrics-collector --config config/config.yaml --start="2025-04-07T00:00:00Z" --end="2025-04-07T23:59:59Z"
```

### Automating Folder Creation for a Date Range

To create folders for all days within a date range without modifying the code, you can use a shell script (Linux/macOS) or batch file (Windows) to automate running the command for each day in the range.

#### Linux/macOS Shell Script

Create a file named `create_folders.sh`:

```bash
#!/bin/bash

# Configuration
CONFIG_FILE="config/config.yaml"
START_DATE="2025-04-06"  # Format: YYYY-MM-DD
END_DATE="2025-04-08"    # Format: YYYY-MM-DD

# Convert dates to seconds since epoch for comparison
start_seconds=$(date -d "$START_DATE" +%s)
end_seconds=$(date -d "$END_DATE" +%s)

# Loop through each day in the range
current_seconds=$start_seconds
while [ $current_seconds -le $end_seconds ]; do
  # Format the current date
  current_date=$(date -d @$current_seconds +%Y-%m-%d)

  # Run the command for the current day
  echo "Processing data for $current_date..."
  ./metrics-collector --config $CONFIG_FILE \
    --start="${current_date}T00:00:00Z" \
    --end="${current_date}T23:59:59Z"

  # Move to the next day (86400 seconds = 1 day)
  current_seconds=$((current_seconds + 86400))
done

echo "Folder creation complete for all days in range."
```

Make the script executable and run it:

```bash
chmod +x create_folders.sh
./create_folders.sh
```

#### Windows Batch File

Create a file named `create_folders.bat`:

```batch
@echo off
setlocal enabledelayedexpansion

:: Configuration
set CONFIG_FILE=config\config.yaml
set START_DATE=2025-04-06
set END_DATE=2025-04-08

:: Convert dates to format usable by Windows
for /f "tokens=1-3 delims=-" %%a in ("%START_DATE%") do (
  set /a START_YEAR=%%a
  set /a START_MONTH=%%b
  set /a START_DAY=%%c
)

for /f "tokens=1-3 delims=-" %%a in ("%END_DATE%") do (
  set /a END_YEAR=%%a
  set /a END_MONTH=%%b
  set /a END_DAY=%%c
)

:: Create a temporary VBScript to help with date calculations
echo Dim StartDate, EndDate, CurrentDate > dateCalc.vbs
echo StartDate = DateSerial(%START_YEAR%, %START_MONTH%, %START_DAY%) >> dateCalc.vbs
echo EndDate = DateSerial(%END_YEAR%, %END_MONTH%, %END_DAY%) >> dateCalc.vbs
echo CurrentDate = StartDate >> dateCalc.vbs
echo Do While CurrentDate ^<= EndDate >> dateCalc.vbs
echo   WScript.Echo Year(CurrentDate) ^& "-" ^& Right("0" ^& Month(CurrentDate), 2) ^& "-" ^& Right("0" ^& Day(CurrentDate), 2) >> dateCalc.vbs
echo   CurrentDate = DateAdd("d", 1, CurrentDate) >> dateCalc.vbs
echo Loop >> dateCalc.vbs

:: Process each date in the range
for /f "delims=" %%d in ('cscript //nologo dateCalc.vbs') do (
  echo Processing data for %%d...
  metrics-collector --config %CONFIG_FILE% --start="%%dT00:00:00Z" --end="%%dT23:59:59Z"
)

:: Clean up
del dateCalc.vbs
echo Folder creation complete for all days in range.
```

Run the batch file:

```batch
create_folders.bat
```

## Important Notes

1. The folder structure is determined by the `--start` parameter, not the `--end` parameter.
2. The `outputDir` in the configuration file (default: "./data") determines the base directory for all Parquet files.
3. If you run a query spanning multiple days but only specify the start day, all data will be stored in folders for that start day, regardless of when the data was actually collected.

## Modifying the Configuration

If you want to change the base output directory, you can modify the `outputDir` setting in your config.yaml file:

```yaml
storage:
  # Directory where Parquet files will be stored
  outputDir: "./custom_data_directory"
```

This will change the base directory for all Parquet files, but the year/month/day/app structure will remain the same.
