# 51Degrees Pipeline API

![51Degrees](https://raw.githubusercontent.com/51Degrees/common-ci/main/images/logo/360x67.png "Data rewards the curious") **Pipeline API** for Go

[Developer Documentation](https://51degrees.com/pipeline-go/index.html?utm_source=github&utm_medium=repository&utm_content=documentation&utm_campaign=go-open-source "developer documentation")
| [![Go Reference](https://pkg.go.dev/badge/github.com/51Degrees/pipeline-go.svg)](https://pkg.go.dev/github.com/51Degrees/pipeline-go)

## Introduction

This repository contains the Go implementation of the 51Degrees Pipeline API.
Individual engines (for example, device detection) are in separate repositories.

The [specification](https://github.com/51Degrees/specifications/blob/main/pipeline-specification/README.md)
is also available on GitHub and is recommended reading if you wish to understand
the concepts and design of this API.

## Dependencies

Go 1.22 or later is required.

The implementation favors the standard library: `log/slog` for logging,
`encoding/json` for configuration, and `net/http` for HTTP and the web
integration. The only third-party dependency is
[`fsnotify`](https://github.com/fsnotify/fsnotify), used to watch data files for
changes.

The [tested versions](https://51degrees.com/documentation/_info__tested_versions.html)
page shows the versions that are currently tested against. The software may run
fine against other versions, but additional caution should be applied.

## Packages

- **core** - The core Pipeline classes such as Pipeline, FlowData, FlowElement
  and Evidence, along with the Pipeline builder and configuration support.
  - *core/typedmap* - The type-safe container used for typed access to results.
- **engines** - Functionality for Aspect Engines, a specialized Flow Element
  with additional features such as caching and data updates.
  - *engines/cache* - The sharded LRU result cache.
  - *engines/dataupdate* - The Data Update Service.
  - *engines/missingproperty* - Missing Property reporting.
- **cloud** - Functionality for making requests to the 51Degrees cloud.
  - *cloud/requestengine* - The Cloud Request Engine, which makes requests to the
    51Degrees cloud service.
  - *cloud/aspectengine* - The base for Cloud Aspect Engines, which deserialize
    the cloud response into typed data.
- **elements** - Common Flow Elements used by multiple other packages.
  - *elements/sequence* - Tracks the session id and callback sequence.
  - *elements/jsonbuilder* - Serializes Flow Data into JSON.
  - *elements/javascriptbuilder* - Packages the JSON into client-side JavaScript.
  - *elements/setheaders* - Gathers HTTP response headers to request more Evidence.
  - *elements/usagesharing* - Shares anonymized usage data with 51Degrees.
  - *elements/translation* - Translates values using YAML translation sources.
- **web** - The `net/http` web integration (middleware and intercepted endpoints).

## Installation

You can either clone this repository and reference the packages locally or
reference the module directly:

```
go get github.com/51Degrees/pipeline-go
```

Then import the packages you need, for example:

```go
import (
    "github.com/51Degrees/pipeline-go/core"
    "github.com/51Degrees/pipeline-go/web"
)
```

## Examples

There are several examples available that demonstrate how to make use of the
Pipeline API in isolation. These are described in the table below.
If you want examples that demonstrate how to use 51Degrees products such as
device detection, then these are available in the corresponding repository and
on our [website](https://51degrees.com/documentation/_examples__device_detection__index.html).

| Example                                                  | Description |
|----------------------------------------------------------|-------------|
| [1. Simple Flow Element](examples/1_simple_element)       | Shows how to create a custom Flow Element that returns a star sign based on a supplied date of birth. |
| [2. On-Premise Engine](examples/2_onpremise_engine)       | Shows how to modify the simple element to make use of the 'engine' functionality and use a custom data file to map dates to star signs rather than relying on hard-coded data. |
| [3. Client-side evidence](examples/3_clientside_evidence) | Shows how to modify the simple element to request the date of birth from the user using client-side JavaScript. |
| [4. Cloud Engine](examples/4_cloud_engine)                | Shows how to modify the simple element to perform the star sign lookup via a cloud service rather than locally. |
| [5. Usage Sharing](examples/5_usage_sharing)              | Shows how to share usage with 51Degrees, configured from an options file. This helps us to keep our products up to date and accurate. |
| [6. Result Caching](examples/6_caching)                   | Shows how the result caching feature works. |

Each example is a runnable program. For example:

```
go run ./examples/1_simple_element
```

## Tests

The tests for each package live alongside the code and can be run with:

```
go test ./...
```

The race detector (`go test -race`) requires a working cgo toolchain, which may
not be available on all platforms.

## Design notes

This implementation follows a "hybrid" approach: public concepts and package
boundaries track the specification and the Java/.NET reference implementations,
while the internals are idiomatic Go. In particular:

- **Typed access via package-level generics.** Go interfaces cannot declare
  type-parameterized methods, so the strongly-typed accessors are functions
  (`core.Get[T]`, `core.GetFromElement[T]`, `typedmap.Get[T]`) layered over a
  non-generic interface, rather than generic methods.
- **Composition over inheritance.** The deep `FlowElement` → `AspectEngine` →
  `CloudAspectEngine` hierarchy of the reference implementations is flattened
  into small interfaces plus embeddable `Base*` structs. Caching and lazy
  loading are applied by an engine-supplied compute function passed to
  `BaseAspectEngine.Execute`, following the strategy pattern recommended in the
  specification's
  [reference-implementation notes](https://github.com/51Degrees/specifications/blob/main/pipeline-specification/reference-implementation-notes.md).

## Project documentation

For complete documentation on the Pipeline API and associated engines, see the
[51Degrees documentation site][Documentation].

[Documentation]: https://51degrees.com/documentation/index.html
