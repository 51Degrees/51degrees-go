# 51Degrees Device Detection and IP Intelligence for Go

<img src=".github/images/logo.png" alt="51Degrees" title="Data rewards the curious" width="160">

**Go modules for Device Detection and IP Intelligence**

Our User-Agent parser for Go supports Device Detection, browser detection, mobile detection, and crawler detection on-premise. IP Intelligence adds IP geolocation, including IP to country data.

It reads User-Agent and User-Agent Client Hints, HTTP request headers that carry browser and device information.

## Install

Install the Device Detection module. See the module table below for the other module paths.

```sh
go get github.com/51Degrees/51degrees-go/device-detection/v4
```

Try the [User-Agent tester](https://51degrees.com/developers/user-agent-tester?utm_source=github&utm_medium=readme&utm_campaign=51degrees-go&utm_content=readme.md&utm_term=install) and [IP tester](https://51degrees.com/developers/ip-tester?utm_source=github&utm_medium=readme&utm_campaign=51degrees-go&utm_content=readme.md&utm_term=install).

Created from `common-go`, `device-detection-go`, and `ip-intelligence-go`, this repository continues their work.

The folder table and product guides below show where to find usage examples and how to build and test the source.

> [!IMPORTANT]
> The Go modules are published from this repository from version 4.5.166, under new module paths (see [What is where](#what-is-where)). The earlier Go repositories contributed their history. Their earlier module paths receive no further releases. Continue development and raise issues here.

## History

| Repository | Contents |
| --- | --- |
| [common-go](https://github.com/51Degrees/common-go) | The common library the engines are built on, with data file updates and file watching |
| [device-detection-go](https://github.com/51Degrees/device-detection-go) | Device Detection |
| [ip-intelligence-go](https://github.com/51Degrees/ip-intelligence-go) | IP Intelligence |

The Pipeline API is included from version 4.5.168.

Each repository is brought in with its full history, so commit SHAs match the original repositories. Open issues and pull requests move here too.

## What is where

| Folder | Go module |
| --- | --- |
| `common` | `github.com/51Degrees/51degrees-go/common/v4` |
| `device-detection` | `github.com/51Degrees/51degrees-go/device-detection/v4` |
| `ip-intelligence` | `github.com/51Degrees/51degrees-go/ip-intelligence/v4` |
| `pipeline` | `github.com/51Degrees/51degrees-go/pipeline/v4` |
| `examples/device-detection`, `examples/ip-intelligence`, `examples/mixed`, `examples/pipeline` | The examples, each a module of its own that isn't published |
| `cxx` | The [51degrees-cxx](https://github.com/51Degrees/51degrees-cxx) submodule, which the engines' C sources are made from |
| `ci` | The scripts the shared 51Degrees CI workflows call |

A module in a folder of this repository is released with a tag named after the folder, such as `device-detection/v4.5.166`, so it is installed with, for example:

```sh
go get github.com/51Degrees/51degrees-go/device-detection/v4
```

Code moving from the old modules changes its imports from `github.com/51Degrees/common-go/v4`, `github.com/51Degrees/device-detection-go/v4` and `github.com/51Degrees/ip-intelligence-go/v4` to the paths above. The packages in them are the same.

## Working on the code

Clone with the submodules:

```sh
git clone --recurse-submodules https://github.com/51Degrees/51degrees-go.git
```

The tests and examples use the free data files that the [data](https://github.com/51Degrees/data) repository links to. The data repository is the `cxx/data` submodule, and its scripts download the files into `cxx/data/device-detection` and `cxx/data/ip-intelligence`.

```sh
sh cxx/data/get-data-files.sh
```

In PowerShell, run `./cxx/data/get-data-files.ps1` instead.

Each module requires the others at a published version, as your own code would. To build and test them against each other instead, create a Go workspace:

```sh
pwsh ci/create-workspace.ps1
```

`go.work` isn't committed. CI uses the workspace only when a published module has changed since the latest release, so the next release is tested with the examples. Otherwise the examples are tested against the published modules, and nothing is released.

The engines compile C sources made from the `cxx` submodule. After moving the submodule to another 51degrees-cxx release, run `pwsh ci/update-amalgamation.ps1` and commit the result. The build fails when they don't match.

## Find out more

- [Device Detection](https://51degrees.com/device-detection?utm_source=github&utm_medium=readme&utm_campaign=51degrees-go&utm_content=readme.md&utm_term=find-out-more)
- [IP Intelligence](https://51degrees.com/ip-intelligence?utm_source=github&utm_medium=readme&utm_campaign=51degrees-go&utm_content=readme.md&utm_term=find-out-more)
- [Documentation](https://51degrees.com/documentation/index.html?utm_source=github&utm_medium=readme&utm_campaign=51degrees-go&utm_content=readme.md&utm_term=find-out-more)
- https://github.com/51Degrees/51degrees-cxx
