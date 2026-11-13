# Mixed examples: Device Detection + IP Intelligence

The examples in this directory **intentionally combine two 51Degrees
products**: [Device Detection](../../device-detection)
and [IP Intelligence](../../ip-intelligence).
References to "Device Detection", "User-Agent" and `.hash` data files here are
deliberate — they belong to the device-detection half of each example.

## A separate Go module

This directory is its own Go module (see [go.mod](go.mod)), so neither engine
depends on the other. In a clone of this repository, the `go.work` at its root
makes it use the engines in the repository, as it does the other examples.
Copied out of it, it uses the published engines its `go.mod` names, as your
own code would.

- Run and test the examples **from this directory**.
- This module is never published or tagged; it exists only for the examples.

## Data files

| Product | Env variable | Data file when it is not set |
|---|---|---|
| IP Intelligence | `_51DEGREES_IPI_PATH` | `51Degrees-LiteV41.ipi` |
| Device Detection | `_51DEGREES_DD_PATH` | `51Degrees-LiteV4.1.hash` |

The [data](https://github.com/51Degrees/data) repository, which is the
`cxx/data` submodule of this repository, links to the free Lite files, and the
examples download a missing one the first time they run. They also show the
properties only the Enterprise data files have, as needing them, and say where
to ask for a copy. To download the Lite files yourself, from the repository
root, run

```bash
sh cxx/data/get-data-files.sh 51Degrees-LiteV4.1.hash 51Degrees-LiteV41.ipi
```

or, in PowerShell,

```
./cxx/data/get-data-files.ps1 51Degrees-LiteV4.1.hash 51Degrees-LiteV41.ipi
```

## Running

```bash
cd examples/mixed
go run ./getting_started_console

# or with paid data files
_51DEGREES_IPI_PATH=../../51Degrees-EnterpriseIpiV41.ipi \
_51DEGREES_DD_PATH=../../TAC-HashV41.hash \
go run ./getting_started_console

# or the web example, then: curl http://localhost:8080/
go run ./getting_started_web
```

## Testing

```bash
cd examples/mixed
go test ./...
```
