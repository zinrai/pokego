# pokego

Poke your processes to reload them! A tool for triggering application reloads via POST requests or SIGHUP signals.

## Features

- **HTTP reload**: Send POST requests to reload endpoints (follows the `/-/reload` convention)
- **SIGHUP reload**: Send SIGHUP signals to processes by name
- **Master targeting**: Match processes by name prefix and signal the master, skipping workers
- **Verbose mode**: Detailed logging for debugging

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
# Send to matching process
$ pokego sighup -name=myapp

# Vrbose output to see matched processes
$ pokego sighup -name=myservice -verbose
```

Options:
- `-name` (required): Process name to match
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

The SIGHUP command selects a target in two steps.

First it matches every process whose name starts with the given `-name` (a prefix match against the executable name). Then, among the matched processes, it signals the **master**: the process whose parent is not itself a match. Prefork servers such as nginx share one name between the master and its workers, and a config reload must reach the master, so workers (whose parent is the master) are skipped.

A running service has exactly one master, so the match normally resolves to a single process. If the prefix spans more than one independent process tree, for example two separate instances or an unrelated daemon sharing the prefix, the command reports the ambiguity and exits without signaling. Narrow `-name` to select one.

## Error Handling

- HTTP requests: Reports non-2xx status codes as errors
- SIGHUP: Reports if no matching processes are found
- Both commands provide clear error messages for troubleshooting

## Why "pokego"?

The name combines "poke" (to prod or nudge something into action) with "Go", reflecting both the action of triggering reloads and the implementation language. Just like poking someone to get their attention, pokego pokes your processes to reload!

## License

This project is licensed under the [MIT License](./LICENSE).
