# 51Degrees Device Detection Engines

![51Degrees](https://raw.githubusercontent.com/51Degrees/51degrees-go/main/.github/images/logo-160.png "Data rewards the curious") **Go Device Detection**

## Introduction

Device Detection and User-Agent parser for Go. It detects the device type and model, browser, operating system, and crawlers from the User-Agent and User-Agent Client Hints, by reading a data file on your server. An alternative to UAParser.js, DeviceAtlas, and WURFL.

This repository contains Go Lite implementation of device detection engine. This provides a light set of device detection features that allow Go users to try and evaluate the capability of 51Degrees device detection solution in Go.

## Supported platforms and architectures

Go Lite implementation is currently supporting the following platforms and architectures:
- Linux 32/64 bit, Intel and ARM processor
- MacOS 64 bit, Intel and ARM processor
- Windows 64bit, Intel

Go version:
- 1.22

Compiles with go 1.22 and higher, the version the Pipeline module asks for. The on-premise engine gives its results
as Aspect Data, the Pipeline module's type, and can be added to a Pipeline (`onpremise.PipelineEngine`), so the module
requires the Pipeline module. [prebid-server](https://github.com/prebid/prebid-server/) uses the earlier
`device-detection-go` module and itself asks for a later Go than 1.22.

## Pre-requisites

### Data File

In order to perform device detection, you will need to use a 51Degrees data file.
A free 'lite' file is available through the [data](https://github.com/51Degrees/data) repository, which is the `cxx/data` submodule of this repository.
To download it into `cxx/data/device-detection`, run one of these from the repository root. The `dd` and `onpremise` examples also download it when it is missing.

```
sh cxx/data/get-data-files.sh 51Degrees-LiteV4.1.hash
```

```
./cxx/data/get-data-files.ps1 51Degrees-LiteV4.1.hash
```

This 'lite' file has a significantly reduced set of properties. To obtain a
file with a more complete set of device properties see the
[51Degrees website](https://51degrees.com/pricing?utm_source=github&utm_medium=readme&utm_campaign=51degrees-go&utm_content=readme.md&utm_term=data-file).

### Software

In order to build use device-detection-go the following are required:
- Powershell Core (7 or above)
- A C compiler that support C11 or above (Gcc on Linux, Clang on MacOS and MinGW-x64 on Windows)
- libatomic - which usually come with default Gcc, Clang installation

### Windows

If you are on Windows, make sure that:
- The path to the `MinGW-x64` `bin` folder is included in the `PATH`. By default, the path should be `C:\msys64\ucrt64\bin`
- Go environment variable `CGO_ENABLED` is set to `1`
```
go env -w CGO_ENABLED=1
```

## Module and Packages

This module name `device-detection-go` at path `github.com/51Degrees/51degrees-go/device-detection/v4`

This Go Lite version contains the following packages:
- `dd` - a lower level API wrapping the C device detection library
- `onpremise` - a higher level Engine API providing device detection and [automatic data file updates](https://51degrees.com/documentation/_device_detection__data_file_updates.html?utm_source=github&utm_medium=readme&utm_campaign=51degrees-go&utm_content=readme.md&utm_term=module-and-packages)

## Build and Usage

### Build steps for all platforms

Import the package as usual and it will get build automatically:

```
import "github.com/51Degrees/51degrees-go/device-detection/v4/dd"
```

### In a Pipeline

`onpremise.NewPipelineEngine` takes the same options as `onpremise.New` and gives the engine as a Pipeline element,
which adds `onpremise.DeviceData` to each Flow Data under the Data Key `device`, as the Device Detection Cloud Engine
does. Used on its own, `Engine.ProcessData` gives the same Aspect Data. To build a Pipeline from a configuration file,
register `onpremise.Factory` under `onpremise.BuilderName` with `core.BuilderRegistry`. The build parameters are the
options' names, such as `DataFile`, `AutoUpdate`, `PerformanceProfile` and `Properties`.

```
{"PipelineOptions": {"Elements": [{"BuilderName": "DeviceDetectionHashEngine",
  "BuildParameters": {"DataFile": "51Degrees-LiteV4.1.hash", "AutoUpdate": false}}]}}
```

### Vendored C Library
An amalgamation is an alternative way to distribute a library's source code using only a few files (as low as one or two).
This go module depends on and ships an amalgamation of the device-detection C-library [device-detection-cxx](https://github.com/51degrees/51degrees-cxx) repository, which is then built automatically by CGo during `go build`.

The amalgamation is produced automatically and regularly by the [Nightly Package Update](https://github.com/51Degrees/51degrees-go/actions/workflows/nightly-pipeline.yml) CI workflow from the device-detection C-library source code.

## Test

Unit tests can be run with `go test ./...` from the root dir.

## APIs

To view APIs and their descriptions, users can use `go doc` in the package directory.
- First navigate to `dd` or `onpremise` dir.
- Then run the below to display all APIs, structures and their descriptions.
```
go doc -all
```

**NOTE**: Not all APIs are available. Ones that are not available include string `TODO: To be implemented` in their descriptions.

## Thread safety

A results instance, being the `dd.ResultsHash` that `Engine.Process`,
`Engine.NewResultsHash` and `dd.NewResultsHash` return, is not safe to use
from several goroutines at once, because reading a value writes into the
native results, and it takes no lock. So a results instance belongs to one
goroutine at a time, and the caller is responsible for thread-safe access to
one it shares, for example with a `sync.Mutex` of its own. A web server makes
one for each request. See the
[results instances rules](https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/features/thread-safety.md?plain=1#L196-L221).

## Examples

Examples can be found in the [examples/device-detection](../examples/device-detection) folder. Users can follow its [README.md](../examples/device-detection/README.md) for details and comments in each examples for how to run them.
