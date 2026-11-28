# 51Degrees Pipeline API

![51Degrees](https://raw.githubusercontent.com/51Degrees/51degrees-go/main/.github/images/logo-160.png "Data rewards the curious") **Pipeline API** for Go

[Developer Documentation](https://51degrees.com/documentation/_pipeline_api__concepts__pipeline_basics.html?utm_source=github&utm_medium=readme&utm_campaign=51degrees-go&utm_content=readme.md&utm_term=developer-documentation "developer documentation")
| [![Go Reference](https://pkg.go.dev/badge/github.com/51Degrees/51degrees-go/pipeline/v4.svg)](https://pkg.go.dev/github.com/51Degrees/51degrees-go/pipeline/v4)

## Introduction

This repository contains the Go implementation of the 51Degrees Pipeline API.
The Device Detection and IP Intelligence modules in this repository don't use it yet.

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

The [tested versions](https://51degrees.com/documentation/_product_summaries__tested_versions.html?utm_source=github&utm_medium=readme&utm_campaign=51degrees-go&utm_content=readme.md&utm_term=tested-versions)
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
  - *cloud/humanconfidence* - The Human Confidence Engine, which gives
    HumanConfidence and HumanConfidenceSource and redeems a sealed Human
    Confidence result. See [Human Confidence](#human-confidence).
- **elements** - Common Flow Elements used by multiple other packages.
  - *elements/sequence* - Tracks the session id and callback sequence.
  - *elements/jsonbuilder* - Serializes Flow Data into JSON.
  - *elements/javascriptbuilder* - Packages the JSON into client-side JavaScript.
  - *elements/setheaders* - Gathers HTTP response headers to request more Evidence.
  - *elements/usagesharing* - Shares anonymized usage data with 51Degrees.
  - *elements/translation* - Translates values using YAML translation sources.
  - *elements/derivedproperty* - Derives new properties from other elements' properties, following the rules in YAML or JSON scripts, such as HumanConfidence and IsGdpr.
- **web** - The `net/http` web integration (middleware and intercepted endpoints).

## Installation

Reference the module directly:

```
go get github.com/51Degrees/51degrees-go/pipeline/v4
```

Then import the packages you need, for example:

```go
import (
    "github.com/51Degrees/51degrees-go/pipeline/v4/core"
    "github.com/51Degrees/51degrees-go/pipeline/v4/web"
)
```

## Examples

There are several examples available that demonstrate how to make use of the
Pipeline API in isolation. These are described in the table below.
If you want examples that demonstrate how to use 51Degrees products such as
device detection, then these are available in this repository's
[examples](../examples) folder and on our [website](https://51degrees.com/documentation/_device_detection__examples__index.html?utm_source=github&utm_medium=readme&utm_campaign=51degrees-go&utm_content=readme.md&utm_term=examples).

| Example                                                  | Description |
|----------------------------------------------------------|-------------|
| [1. Simple Flow Element](../examples/pipeline/1_simple_element)       | Shows how to create a custom Flow Element that returns a star sign based on a supplied date of birth. |
| [2. On-Premise Engine](../examples/pipeline/2_onpremise_engine)       | Shows how to modify the simple element to make use of the 'engine' functionality and use a custom data file to map dates to star signs rather than relying on hard-coded data. |
| [3. Client-side evidence](../examples/pipeline/3_clientside_evidence) | Shows how to modify the simple element to request the date of birth from the user using client-side JavaScript. |
| [4. Cloud Engine](../examples/pipeline/4_cloud_engine)                | Shows how to modify the simple element to perform the star sign lookup via a cloud service rather than locally. |
| [5. Usage Sharing](../examples/pipeline/5_usage_sharing)              | Shows how to share usage with 51Degrees, configured from an options file. This helps us to keep our products up to date and accurate. |
| [6. Result Caching](../examples/pipeline/6_caching)                   | Shows how the result caching feature works. |
| [7. Derived Property](../examples/pipeline/7_derived_property)        | Shows how the Derived Property Element computes a property from properties other elements have produced, with a script held in code, a script file beside the example and the same Pipeline built from a configuration file, and what a script gives when a source property is missing. |

Each example is a runnable program in [examples/pipeline](../examples/pipeline), a module of its own that uses
the published Pipeline module. For example, from that folder:

```
go run ./1_simple_element
```

## Tests

The tests for each package live alongside the code and can be run with:

```
go test ./...
```

The race detector (`go test -race`) requires a working cgo toolchain, which may
not be available on all platforms.

## Web integration

The `web` package integrates the Pipeline into a `net/http` application.
`AssemblePipeline` builds a web-ready Pipeline, adding the Sequence, Set Headers,
JSON Builder and JavaScript Builder elements in the order required by the
specification (each can be disabled). The `Middleware` then populates Evidence
from each request, processes it, applies any response headers requested by the
Set Headers Element, exposes the result via `web.FromContext`, and serves the
intercepted client-side endpoints `/51Degrees.core.js` and `/51dpipeline/json`.

```go
pipeline, err := web.AssemblePipeline(web.PipelineConfig{}, myElements...)
if err != nil {
    log.Fatal(err)
}
defer pipeline.Close()

mw := web.NewMiddleware(pipeline)
http.ListenAndServe(":5151", mw.Handler(myAppHandler))

// Inside myAppHandler:
//   flowData, _ := web.FromContext(r.Context())
//   A Flow Data cannot be used by two goroutines at once, so copy the
//   values that work on another goroutine needs, and pass those:
//   device, _ := flowData.Get("device")
//   isMobile, _ := device.Get("ismobile")
//   go record(isMobile)
```

The example at [examples/pipeline/3_clientside_evidence](../examples/pipeline/3_clientside_evidence)
stands up the middleware and exercises the full client-side round-trip.

## Human Confidence

HumanConfidence says how confident the cloud service is that a person is
using the browser, as `High`, `Medium` or `Low`. HumanConfidenceSource says
how it was worked out: `Passive` from the request alone, or `Active` where the
browser also ran the 51Degrees browser tests. Both are in the `derived`
Element Data.

After the browser tests run, the page gets HumanConfidence sealed, so the
browser can't read or change it. The page sends it back to your server as the
`humanconfidence` parameter, which the web integration adds as the Evidence
`query.humanconfidence`. Add the Human Confidence Engine
(`cloud/humanconfidence`) after the Cloud Request Engine, and after any
Derived Property Element. On a request that carries a sealed result, the
Cloud Request Engine sends it in its one request, and the engine gives
HumanConfidenceSource `Active` and HumanConfidence the redeemed Value. Your
code only reads the two Properties and never redeems anything itself. A
sealed result redeems once, within 10 seconds, and needs the License Key of
the same account set on the Cloud Request Engine. Where it isn't redeemed,
for any reason, HumanConfidence is `Medium`.

A page whose request names HumanConfidence also gets the browser tests, so the
client script's complete handlers run later than they otherwise would, by the
time the tests take and one more request. A page can read the first
response's Properties sooner through its change handlers.

Where a page sets a Content Security Policy, it must allow these for the
browser tests to run. Otherwise HumanConfidenceSource stays `Passive`.

| Directive | Must allow |
|-----------|------------|
| `script-src` | The cloud service's host, and `'unsafe-eval'` |
| `connect-src` | The cloud service's host |
| `worker-src` | `blob:` |

Each sealed result is bound to the page that asked for it by a transaction.
The Cloud Request Engine issues one for each page, keeps it on your server,
and sends it on every request the page makes, so the cloud service binds the
page's sealed result to it. It sends it once more to redeem that sealed
result, and then forgets it. A page is known by its session id, which the
Sequence Element gives it (`web.AssemblePipeline` adds that element) and the
client script sends with every request. A transaction is kept for 30 minutes,
and your code never sees it. Where no transaction is kept for a request, as
for a page that loads the cloud service's script itself, the sealed result
is sent with none, and the cloud service redeems one made with none.

No error or log entry the Cloud Request Engine writes holds the License Key,
a sealed result or a transaction, in whole or in part.

The engine follows the
[Human Confidence](https://github.com/51Degrees/specifications/blob/81c1e4df5c7eb923d88ec3b1926d562b0a754ad3/pipeline-specification/features/human-confidence.md)
specification's draft, except that the draft gives `Medium` where no
transaction is kept, and the engine sends the sealed result with none, as the
cloud service allows. `requestengine.RedeemWithoutTransaction` decides that.

## Thread safety

A Flow Data is used by one goroutine at a time, and is not safe to use from
several at once, apart from the members of a parallel group, which the
Pipeline runs itself. Once `Process` has returned, a Flow Data may pass to
another goroutine, which then uses it alone.

Using a Flow Data from another goroutine while `Process` runs is a data race.
It can end the whole program with a fatal error, such as
`fatal error: concurrent map read and map write`, which `recover` does not
catch, rather than return an error.

The Pipeline adds no lock for this, to keep processing fast for the usual case
of one goroutine per request. A caller that shares a Flow Data across
goroutines is responsible for any locking.

Element Data is written only by the Flow Element that provides it, or the Flow
Elements that share it, and only while they process. Once processing has
finished, a write to the Flow Data
panics with `core.ErrFlowDataFinished`, and a write to Element Data panics with
`core.ErrElementDataFinished`. Two Flow Elements that write one Element Data
instance must not be members of the same parallel group, so two Derived
Property Elements MUST NOT be members of the same parallel group, nor two
Translation Engines.

Once its Flow Element has finished, Element Data may be read from several
goroutines at once, except Element Data that reads its values from native
results, such as the Device Detection `DeviceData` and the IP Intelligence
`Data`. That Element Data takes no lock and is not safe to use from several
goroutines at once. Members of a parallel group after the Device Detection
or IP Intelligence on-premise Engine that read its Element Data are
responsible for thread-safe access to it. A `sync.Mutex` the readers share
protects it only when every reader takes it, and the Flow Elements in this
repository, such as the JSON Builder, the JavaScript Builder, the Set
Headers Element, the Derived Property Element, the Translation Engine and
the IP Intelligence Countries Translation Engine, take no lock when they
read other Element Data. So a member that reads that
Engine's Element Data must not share a parallel group with any of these
Flow Elements that also reads it, and the other members of the group that
read it must all take the same lock.

`FlowData.Close` returns its Element Data to a pool, to be used again by a
later Flow Data. Call it once the Flow Data and its values are no longer
needed. Keeping a reference to Element Data after its Flow Data is closed is
not supported, as the instance may by then hold another request's values. The
web middleware closes the Flow Data when the request ends, that is when the
handler it wraps returns, so copy
the values that work outliving the request needs before the handler returns.
Put a time limit such as `http.TimeoutHandler` around the middleware, not
inside it, because the handler that a time limit gives up on keeps running
after the time limit has returned. Code still reading a closed Flow Data reads
values that are gone or belong to another request, and a read that meets a
write can end the program with a fatal error that `recover` does not catch. A
custom Flow Element that gives the same Element Data to more than one Flow
Data, or keeps it across requests, marks it with `core.MarkShared`, so it never
goes back to a pool.

This follows the Pipeline specification's
[thread-safety page](https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/features/thread-safety.md).

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
  into small interfaces plus embeddable `Base*` structs. Caching is applied
  around an engine-supplied compute function passed to
  `BaseAspectEngine.Execute`, which runs it on the calling goroutine, following
  the strategy pattern recommended in the specification's
  [reference-implementation notes](https://github.com/51Degrees/specifications/blob/main/pipeline-specification/reference-implementation-notes.md).
  `SetLazyLoading` is deprecated and has no effect, as the specification's
  [lazy loading](https://github.com/51Degrees/specifications/blob/a90ba516a3bfadc58dd7a51535d83af6bdaed291/pipeline-specification/advanced-features/lazy-loading.md)
  page says. Long-running work belongs in a
  [lazy property](https://github.com/51Degrees/specifications/blob/a90ba516a3bfadc58dd7a51535d83af6bdaed291/pipeline-specification/features/thread-safety.md#lazy-properties)
  of a Flow Element's own Element Data. The Cloud Request Engine works this
  way. It starts its HTTP request while it processes and returns without
  waiting, and every read of its `json-response` waits for that request.
  A request that fails is recorded as the Cloud Request Engine's error by the
  first Flow Element that reads `json-response` while the Flow Data is
  processed. Once processing has finished, the failure reaches only the
  reader, as the error of the read. Closing the engine, or the Pipeline that
  holds it, waits for every request the engine has started.

## Project documentation

For complete documentation on the Pipeline API and associated engines, see the
[51Degrees documentation site][Documentation].

[Documentation]: https://51degrees.com/documentation/index.html?utm_source=github&utm_medium=readme&utm_campaign=51degrees-go&utm_content=readme.md&utm_term=project-documentation
