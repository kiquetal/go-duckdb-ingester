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
