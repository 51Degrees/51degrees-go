# 51Degrees Ip Intelligence API Examples

![51Degrees](https://51degrees.com/img/logo.png?utm_source=github&utm_medium=readme&utm_campaign=ip-intelligence-go&utm_content=examples-readme.md&utm_term=51degrees-ip-intelligence-api-examples "Data rewards the curious")
**IP Intelligence Go Examples**

## Introduction

This directory contains usage examples of the [ip-intelligence-go](https://github.com/51degrees/ip-intelligence-go) module.

## Pre-requisites

To run these examples you will need a data file and example evidence. To fetch these assets follow the instructions in
the [ip-intelligence-data](https://github.com/51Degrees/ip-intelligence-data) repo and put in the root of this repository.

### Software

The dependency library `ip-intelligence-go` uses CGO, so make sure `CGO_ENABLED=1` - this is the default, unless you override it.

### Windows

If you are on Windows, make sure that:

- The path to the `MinGW-x64` `bin` folder is included in the `PATH`. By default, the path should be
  `C:\msys64\ucrt64\bin`
- Go environment variable `CGO_ENABLED` is set to `1`

```
go env -w CGO_ENABLED=1
```

## Running the examples

- All examples under `ipi_onpremise` directory are console program examples and are run using `go run`.

Below is a table that describes the examples:

| Example                       | Description                                                                                                               |
|-------------------------------|---------------------------------------------------------------------------------------------------------------------------|
| getting_started               | An example showing how to initialize the IPI engine, minimum required parameters, calling the engine and printing the result |
| offline_processing            | Example showing how to get values from the engine in weighted value format; writing the obtained values to a yaml file    |
| performance                   | A benchmarking example to measure the speed of data processing in single and multi-threaded modes                         |
| reload_from_file              | An example that demonstrates how a data file can be reloaded while serving IP Intelligence requests                       |
| update_polling_interval       | An example doing periodic polling for the updated data file                                                               |
| mixed/getting_started_console | A command line example featuring both device detection and IP intelligence                                                |
| mixed/getting_started_web     | A web example featuring both device detection and IP intelligence                                                |

The `mixed` examples intentionally combine Device Detection and IP Intelligence
and live in their own Go module — see [mixed/README.md](mixed/README.md) for how
to run them (they cannot be run with `go run` from the repository root).

## Run examples

```
go run ./examples/example_dir
```
For further details of how to run each example, please read more in the comment section located at the top of each example file.

To provide a different path to a data file or evidence file use environment variables, e.g.
```bash
DATA_FILE=../51Degrees-EnterpriseIpiV41.ipi go run ./examples/getting_started
DATA_FILE=../51Degrees-EnterpriseIpiV41.ipi EVIDENCE_YAML=../20000_ipi_evidence_records.yml go run ./examples/offline_processing
DATA_FILE=../51Degrees-EnterpriseIpiV41.ipi EVIDENCE_YAML=../20000_ipi_evidence_records.yml go run ./examples/update_polling_interval
```
