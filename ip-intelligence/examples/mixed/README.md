# Mixed examples: Device Detection + IP Intelligence

The examples in this directory **intentionally combine two 51Degrees
products**: [device-detection-go](https://github.com/51Degrees/device-detection-go)
and [ip-intelligence-go](https://github.com/51Degrees/ip-intelligence-go).
References to "Device Detection", "User-Agent" and `.hash` data files here are
deliberate — they belong to the device-detection half of each example.

## A separate Go module

This directory is its own Go module (see [go.mod](go.mod)). That keeps the
`device-detection-go` dependency out of the root `ip-intelligence-go` module,
so consumers of the IP Intelligence library do not pull in device detection.
A `replace` directive points the IP Intelligence dependency at the enclosing
repository, so these examples always build against the local source.

Two consequences:

- Run and test the examples **from this directory**, not the repository root —
  `go run ./examples/mixed/...` and `go test ./examples/...` from the root do
  not reach into a nested module.
- This module is never published or tagged; it exists only for the examples.

## Data files

| Product | Env variable | Data file |
|---|---|---|
| IP Intelligence | `DATA_FILE` | `51Degrees-EnterpriseIpiV41.ipi` (or Lite equivalent) |
| Device Detection | `DD_DATA_FILE` | a `.hash` file, e.g. `51Degrees-LiteV4.1.hash` or `TAC-HashV41.hash` |

See [ip-intelligence-data](https://github.com/51Degrees/ip-intelligence-data)
and [device-detection-data](https://github.com/51Degrees/device-detection-data)
for obtaining the files.

## Running

```bash
cd examples/mixed
DATA_FILE=../../51Degrees-EnterpriseIpiV41.ipi \
DD_DATA_FILE=../../51Degrees-LiteV4.1.hash \
go run ./getting_started_console

# or the web example, then: curl http://localhost:8080/
go run ./getting_started_web
```

## Testing

```bash
cd examples/mixed
go test ./...
```
