# 51Degrees Go

![51Degrees](https://51degrees.com/img/logo.png?utm_source=github&utm_medium=readme&utm_campaign=51degrees-go&utm_content=readme.md&utm_term=51degrees-go "Data rewards the curious") **Go modules for Device Detection and IP Intelligence**

> [!IMPORTANT]
> The Go modules are published from this repository from version 4.5.166, under new module paths (see [What is where](#what-is-where)). The Go repositories listed below were merged into it, and their modules get no further releases.

## Where it came from

| Repository | Contents |
| --- | --- |
| [common-go](https://github.com/51Degrees/common-go) | The common library the engines are built on: data file updates and file watching |
| [device-detection-go](https://github.com/51Degrees/device-detection-go) | Device Detection |
| [ip-intelligence-go](https://github.com/51Degrees/ip-intelligence-go) | IP Intelligence |

Each repository is brought in with its full history, so commit SHAs match the original repositories. Open issues and pull requests move here too.

## What is where

| Folder | Go module |
| --- | --- |
| `common` | `github.com/51Degrees/51degrees-go/common/v4` |
| `device-detection` | `github.com/51Degrees/51degrees-go/device-detection/v4` |
| `ip-intelligence` | `github.com/51Degrees/51degrees-go/ip-intelligence/v4` |
| `examples/device-detection`, `examples/ip-intelligence`, `examples/mixed` | The examples, each a module of its own that isn't published |
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

Each module requires the others at a published version, as your own code would. To build and test them against each other instead, create a Go workspace:

```sh
pwsh ci/create-workspace.ps1
```

`go.work` isn't committed. CI uses the workspace only when a published module has changed since the latest release, so the next release is tested with the examples. Otherwise the examples are tested against the published modules, and nothing is released.

The engines compile C sources made from the `cxx` submodule. After moving the submodule to another 51degrees-cxx release, run `pwsh ci/update-amalgamation.ps1` and commit the result. The build fails when they don't match.
