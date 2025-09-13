# pokego

Poke your processes to reload them! A tool for triggering application reloads via POST requests or SIGHUP signals.

## Features

- **HTTP reload**: Send POST requests to reload endpoints (follows the `/-/reload` convention)
- **SIGHUP reload**: Send SIGHUP signals to processes by name
- **Process matching**: Find processes by exact name or partial match
- **Batch operations**: Send signals to all matching processes with `-all`
- **Verbose mode**: Detailed logging for debugging

## Installation

```bash
$ go install github.com/zinrai/pokego@latest
```

## Usage

### HTTP Reload

Send a POST request to trigger application reload:

```bash
# Standard reload endpoint
$ pokego http -url=http://localhost:8080/-/reload

# Custom reload endpoint
$ pokego http -url=http://localhost:9090/api/reload

# With custom timeout and verbose output
$ pokego http -url=http://localhost:8080/-/reload -timeout=10s -verbose
```

Options:

- `-url` (required): Full URL to send POST request to
- `-timeout`: Request timeout (default: 30s)
- `-verbose`: Enable detailed logging

### SIGHUP Reload

Send SIGHUP signal to processes by name:

```bash
# Send to first matching process
$ pokego sighup -name=myapp

# Send to all matching processes
$ pokego sighup -name=custom-exporter -all

# Verbose output to see matched processes
$ pokego sighup -name=myservice -verbose
```

Options:
- `-name` (required): Process name to match
- `-all`: Send signal to all matching processes (default: first match only)
- `-verbose`: Show detailed process information

## Integration with igotifier

`pokego` is designed to work seamlessly with [igotifier](https://github.com/zinrai/igotifier):

```bash
# Watch config file and reload custom application via SIGHUP
igotifier -path="/etc/myapp/config.yaml" -exec="pokego sighup -name=myapp"

# Watch config directory and reload Prometheus
igotifier -path="/etc/prometheus" -exec="pokego http -url=http://localhost:9090/-/reload"

# Development environment hot reload
igotifier -path="./config" -exec="pokego http -url=http://localhost:3000/-/reload"
```

## Process Matching

The SIGHUP command matches processes in two ways:
1. **Exact match**: Process name exactly matches the provided name
2. **Partial match**: Process name contains the provided string

For example, `-name=myapp` will match:
- `myapp` (exact)
- `myapp-worker` (contains)
- `/usr/local/bin/myapp` (contains)

## Error Handling

- HTTP requests: Reports non-2xx status codes as errors
- SIGHUP: Reports if no matching processes are found
- Both commands provide clear error messages for troubleshooting

## Why "pokego"?

The name combines "poke" (to prod or nudge something into action) with "Go", reflecting both the action of triggering reloads and the implementation language. Just like poking someone to get their attention, pokego pokes your processes to reload!

## License

This project is licensed under the [MIT License](./LICENSE).
