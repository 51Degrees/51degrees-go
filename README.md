# pipeline-go

A Go implementation of the [51Degrees Pipeline API](https://github.com/51Degrees/specifications/blob/main/pipeline-specification/README.md).

The Pipeline API is a framework within which input data (**Evidence**) is
transformed and enriched by a sequence of **Flow Elements** to produce output
data (**Element Data**) that an application can consume. This repository
implements the language-agnostic Pipeline specification in idiomatic Go.

> This module contains the generic Pipeline framework and the 51Degrees-generic
> elements only. Concrete services such as Device Detection and IP Intelligence
> are intended to live in their own repositories that depend on this one, as in
> the [Java](https://github.com/51Degrees/pipeline-java) and
> [.NET](https://github.com/51Degrees/pipeline-dotnet) reference implementations.

## Status

Implements the full Pipeline specification: core, the Aspect Engine features
layer, cloud engines, the generic Flow Elements, and `net/http` web integration.
Standard library only, plus [`fsnotify`](https://github.com/fsnotify/fsnotify)
for data-file watching.

## Installation

```sh
go get github.com/51Degrees/pipeline-go
```

Requires Go 1.22 or later.

## Quick start

```go
package main

import (
    "fmt"

    "github.com/51Degrees/pipeline-go/core"
)

func main() {
    pipeline, _ := core.NewPipelineBuilder().
        AddFlowElement(NewMyElement()).
        Build()
    defer pipeline.Close()

    fd := pipeline.CreateFlowData()
    fd.AddEvidence("header.user-agent", "...")
    if err := fd.Process(); err != nil {
        // handle error (unless SuppressProcessExceptions is enabled)
    }

    data, _ := fd.Get("my-element")
    value, _ := data.Get("my-property")
    fmt.Println(value)
}
```

See [`examples/`](examples) for runnable programs covering every required
example: a simple element, an on-premise engine with an external data file, an
element fed by client-side JavaScript, a cloud engine, usage sharing configured
from a file, and a cached engine. Run any of them with, for example:

```sh
go run ./examples/1_simple_element
```

## Package layout

| Package | Responsibility |
|---------|----------------|
| `core` | Pipeline, FlowData, FlowElement, Evidence, builders, configuration, errors, value types |
| `core/typedmap` | Generic type-safe key/value container underpinning typed access |
| `engines` | Aspect Engine base, AspectPropertyValue (null support), caching and lazy-loading integration |
| `engines/cache` | Sharded LRU cache and deterministic cache-key generation |
| `engines/dataupdate` | Data Update Service (HTTP polling, file watching, atomic file replace) |
| `engines/missingproperty` | Missing-property error messages |
| `cloud/requestengine` | Cloud Request Engine (single HTTP call per Flow Data) |
| `cloud/aspectengine` | Cloud Aspect Engine base (JSON → typed data) |
| `elements/sequence` | Sequence Element (session id and sequence counter) |
| `elements/jsonbuilder` | JSON Builder Element |
| `elements/javascriptbuilder` | JavaScript Builder Element (Mustache template) |
| `elements/setheaders` | Set Headers Element |
| `elements/usagesharing` | Usage Sharing Element |
| `elements/translation` | Translation Engine |
| `web` | `net/http` web integration (middleware, intercepted endpoints) |

## Design notes

This implementation follows a "hybrid" approach: public concepts and package
boundaries track the specification and the Java/.NET reference implementations,
while the internals are idiomatic Go. In particular:

- **Typed access via package-level generics.** Go interfaces cannot declare
  type-parameterized methods, so the strongly-typed accessors are functions
  (`core.Get[T](fd)`, `core.GetFromElement[T](fd, e)`, `typedmap.Get[T]`) layered
  over a non-generic interface, rather than generic methods.
- **Composition over inheritance.** The deep `FlowElement → AspectEngine →
  CloudAspectEngine` hierarchy of the reference implementations is flattened into
  small interfaces plus embeddable `Base*` structs. Caching and lazy loading are
  applied by an engine-supplied compute function passed to
  `BaseAspectEngine.Execute`, following the strategy pattern recommended in the
  specification's
  [reference-implementation notes](https://github.com/51Degrees/specifications/blob/main/pipeline-specification/reference-implementation-notes.md).
- **Standard library first.** Logging uses `log/slog`, configuration uses
  `encoding/json`, HTTP uses `net/http`. The only third-party dependency is
  `fsnotify` for file-system watching, which the standard library cannot provide.

## Configuration from a file

Pipelines can be built from a JSON configuration file via a builder registry,
keeping configuration consistent with the other language implementations:

```go
opts, _ := core.LoadPipelineOptions("pipeline.json")
registry := core.NewBuilderRegistry()
registry.Register("ShareUsage", usagesharing.Factory)
// ... register other element factories ...
pipeline, _ := registry.BuildFromConfiguration(opts, nil)
```

## Web integration

```go
pipeline, _ := web.AssemblePipeline(web.PipelineConfig{}, myElements...)
mw := web.NewMiddleware(pipeline)
http.ListenAndServe(":8080", mw.Handler(myAppHandler))
```

`AssemblePipeline` adds the Sequence, Set Headers, JSON Builder and JavaScript
Builder elements in the order required by the specification (each can be
disabled). The middleware populates Evidence from the request, processes it,
applies response headers requested by the Set Headers Element, exposes the
result via `web.FromContext`, and serves the `/51Degrees.core.js` and
`/51dpipeline/json` endpoints with appropriate caching headers.

## Testing

```sh
go test ./...
```

The race detector (`go test -race`) requires a working cgo toolchain, which may
not be available on all platforms.

## License

See the reference implementations for licensing; this code is provided under the
same terms as the 51Degrees Pipeline API.
