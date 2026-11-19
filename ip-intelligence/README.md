# 51Degrees IP Intelligence API

![51Degrees](https://raw.githubusercontent.com/51Degrees/51degrees-go/main/.github/images/logo-160.png "Data rewards the curious") **IP Intelligence in GO**

## Introduction

IP Intelligence and IP geolocation for Go. It finds an IP address's likely location (country, region, town, and coordinates), connection type, and network, by reading a data file on your server or using the 51Degrees cloud service. An alternative to MaxMind and IPinfo.

This repository contains a Go implementation of the IP Intelligence engine.
This module allows you to call functions for determining to set up with an IP Intelligence engine and begin using it to process IP addresses.

## Module Structure

**Module**: `ip-intelligence-go` at path `github.com/51Degrees/51degrees-go/ip-intelligence/v4`

This Go version contains the following packages:
- `ipi_interop` - a lower level API wrapping the C IP Intelligence library
- `ipi_onpremise` - a higher level Engine API providing IP Intelligence library and file update functions

### Minimum Go Version
- Go 1.22

## Setup Instructions

### Data File Setup

To process IP addresses, you will need to use the 51Degrees data file.
A free Lite data file is available through the [data](https://github.com/51Degrees/data) repository, which is the `cxx/data` submodule of this repository, or [contact us](https://51degrees.com/contact-us?utm_source=github&utm_medium=readme&utm_campaign=51degrees-go&utm_content=readme.md&utm_term=data-file-setup) to get an Enterprise data file.
To download the Lite file into `cxx/data/ip-intelligence`, run one of these from the repository root.

```
sh cxx/data/get-data-files.sh 51Degrees-LiteV41.ipi
```

```
./cxx/data/get-data-files.ps1 51Degrees-LiteV41.ipi
```

### Windows Configuration

If you are on Windows, make sure that:
- The path to the `MinGW-x64` `bin` folder is included in the `PATH`. By default, the path should be `C:\msys64\ucrt64\bin`
- Go environment variable `CGO_ENABLED` is set to `1`

```
bash go env -w CGO_ENABLED=1
```

## Build and Usage

### Installation

Import the package as usual and it will get build automatically:

```
go import "github.com/51Degrees/51degrees-go/ip-intelligence/v4/ipi_onpremise"
```

### In a Pipeline

`ipi_onpremise.NewPipelineEngine` takes the same options as `ipi_onpremise.New` and gives the engine as a Pipeline
element, which adds `ipi_onpremise.Data` to each Flow Data under the Data Key `ip`, in the form the IP Intelligence
Cloud Engine gives its data in. The Data reads each Property from the data file's results only when it is requested, and
holds those results in memory the Go garbage collector cannot see until it is released. Closing the Flow Data does not
release them, so call `Close` on the Data when you have finished with the Flow Data, which releases the results at once:

```go
if data, err := core.Get[*ipi_onpremise.Data](flowData); err == nil {
	defer data.Close()
}
```

If a Data is not closed, a finalizer releases its results once nothing holds it, but only when the garbage collector
next runs, which can be too late under load. A Data is not safe to use from several goroutines at once, because reading
a value writes into its results, and it takes no lock, so the caller is responsible for any locking. The engine refuses a results cache, and its `SetCache` returns an error,
because a cached Data would be read after its results were released. Used on its own, `Engine.ProcessData` gives the
same Data, and the caller calls `Close` on it when it has finished with it. To build a Pipeline from a configuration
file, register `ipi_onpremise.Factory` under `ipi_onpremise.BuilderName` with `core.BuilderRegistry`. The build
parameters are the options' names, such as `DataFile`, `AutoUpdate`, `PerformanceProfile` and `Properties`.

```
{"PipelineOptions": {"Elements": [{"BuilderName": "IpiOnPremiseEngine",
  "BuildParameters": {"DataFile": "51Degrees-LiteV41.ipi", "AutoUpdate": false}}]}}
```

### Vendored C Library

Implementation wraps an amalgamated C library from [ip-intelligence-cxx](https://github.com/51Degrees/51degrees-cxx) repo

## Thread safety

A results instance, being the `ipi_interop.ResultsIpi` that
`Engine.NewResultsIpi` and `ipi_interop.NewResultsIpi` return and that
`Engine.ProcessWithResults` reuses, is not safe to use from several goroutines
at once, because reading a value writes into the native results, and it takes
no lock. So a results instance belongs to one goroutine at a time, and the
caller is responsible for thread-safe access to one it shares, for example
with a `sync.Mutex` of its own. A web server makes one for each request. The
`Data` that `Engine.ProcessData` returns reads its values from a results
instance of its own, so it is not safe to use from several goroutines at once
either, and the caller is responsible for thread-safe access to one it shares
in the same way. See the
[results instances rules](https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/features/thread-safety.md?plain=1#L196-L221).

## Development

### Testing

Unit tests can be run with `go test ./...` from the root dir.

### API Documentation

To view APIs and their descriptions, users can use `go doc` in the package directory.
- First navigate to `ipi_interop` or `ipi_onpremise` dir.
- Then run the below to display all APIs, structures and their descriptions.


```bash
go doc -all
```

### Examples

Examples can be found in the [examples/ip-intelligence](../examples/ip-intelligence) folder.
Users can follow its [README.md](../examples/ip-intelligence/README.md) for details and comments in each examples for how to run them.
