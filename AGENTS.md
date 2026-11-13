# Working in this repository

If you have been asked to get an example running, follow
[Run the examples](#run-the-examples). It gives every step in order, from a
fresh clone to the output. The sections after it are notes on what is not
obvious from the code, for changing it. Keep them short and current. The
layout, the workspace and the C sources are described in the
[README](README.md).

## Run the examples

Device Detection's `dd/getting_started` is the quickest example to show,
because its data file is about 35 MB. IP Intelligence's `getting_started`
needs a download of about 230 MB that unpacks to about 460 MB, so run it
second. Run each command from the repository root unless the step says
otherwise.

Each folder under `examples` is a Go module of its own that requires the
published modules at the version in its `go.mod`, as a customer's code would.
In a clone, the committed `go.work` at the root makes the examples use the
modules in this repository instead, so `go run` compiles their C code through
cgo with no settings, and nothing needs building first. Copied out of the
repository, an example downloads the published modules its `go.mod` names.

### Prerequisites

- Git.
- Go 1.22 or later, because the IP Intelligence and Pipeline modules ask for
  1.22. The Device Detection module on its own asks for 1.19. CI tests Go 1.23
  and 1.24 (`ci/options.json`).
- A C compiler for cgo, being GCC on Linux, Clang on macOS and MinGW-x64 on
  Windows, with `libatomic`, which usually comes with GCC and Clang.
- On Windows, the MinGW-x64 `bin` folder on the `PATH` (by default
  `C:\msys64\ucrt64\bin`) and cgo turned on, as the examples' READMEs say.

  ```
  go env -w CGO_ENABLED=1
  ```

- Network access to GitHub, to the Go module proxy for the modules the
  examples use, and to `distributor.51degrees.com` for the data files.
- For the data files, `curl`, `gzip` and `md5sum` (`md5` on macOS) on Linux
  and macOS, where the examples download with the shell script, and Windows
  PowerShell 5.1 or PowerShell 7 on Windows, where they use the PowerShell
  script.

Clone the repository with its submodules.

```
git clone --recurse-submodules https://github.com/51Degrees/51degrees-go.git
cd 51degrees-go
```

In a clone made without `--recurse-submodules`, fetch the submodules.

```
git submodule update --init --recursive
```

### Data files

The examples use the free Lite data files, which need no licence key. They are
not stored in this repository. The `cxx` submodule holds the
[data](https://github.com/51Degrees/data) repository as `cxx/data`, which has a
link to each file, and a script downloads them into `cxx/data/device-detection`
and `cxx/data/ip-intelligence`. Download them before running the Device
Detection example, for the reason "Run" gives.

On Linux and macOS, run the shell script.

```
sh cxx/data/get-data-files.sh
```

On Windows, run the PowerShell script. Windows PowerShell can refuse to run a
script file under the policy Windows sets by default, so run it the way the
examples do.

```
powershell -NoProfile -ExecutionPolicy Bypass -File cxx/data/get-data-files.ps1
```

Where PowerShell 7 is installed, `pwsh cxx/data/get-data-files.ps1` does the
same on any system. Either script downloads every free file, and keeps a file
that is already there. To download one file, give its name, as in
`sh cxx/data/get-data-files.sh 51Degrees-LiteV4.1.hash`.

#### Check the files

- `cxx/data/device-detection/51Degrees-LiteV4.1.hash` is more than 10 MB.
- `cxx/data/ip-intelligence/51Degrees-LiteV41.ipi` is more than 100 MB.

### Run

Run each example from its own module folder, because there is no `go.mod` at
the repository root. The first run downloads the modules and compiles the C
library, so it takes longer than later runs.

The Device Detection example finds `51Degrees-LiteV4.1.hash` by searching
every folder below the folder two above the one it runs in, and takes the
first file with that name in lexical order (`GetFilePathByName` in
`examples/device-detection/dd/example_base.go`, which calls `dd.GetFilePath`
in the Device Detection module). From `examples/device-detection` that is the
repository root, so the search reaches `cxx/data/device-detection`. The
example downloads the file only when the search finds no file with that name
at all, so download it first, as "Data files" says.

```
cd examples/device-detection
go run ./dd/getting_started
cd ../..
```

The IP Intelligence example reads the data file named by the `_51DEGREES_IPI_PATH`
environment variable, and otherwise looks for `51Degrees-EnterpriseIpiV41.ipi`
in the folder it runs in, without searching
(`examples/ip-intelligence/common/example_base.go`). So name the Lite file.
When that file is missing, the example downloads it first, because the data
repository links to it (`GetDataFile` in
`examples/ip-intelligence/common/file.go`).

```
cd examples/ip-intelligence
_51DEGREES_IPI_PATH=../../cxx/data/ip-intelligence/51Degrees-LiteV41.ipi go run ./getting_started
cd ../..
```

In PowerShell, set the variable first.

```
cd examples/ip-intelligence
$env:_51DEGREES_IPI_PATH = '../../cxx/data/ip-intelligence/51Degrees-LiteV41.ipi'
go run ./getting_started
cd ../..
```

### Correct output

When an example downloads its data file, it first logs
`<file> is missing, so it is being downloaded by running` with the command on
the next line, and the script's `Downloading` and `Saved` lines follow.

The Device Detection example prints these lines. It compares its output with
this text itself, and stops with `Output does not match expected.` and exit
code 1 when the two differ. CI runs it with the Lite file.

```
Mobile User-Agent: Mozilla/5.0 (iPhone; CPU iPhone OS 7_1 like Mac OS X) AppleWebKit/537.51.2 (KHTML, like Gecko) Version/7.0 Mobile/11D167 Safari/9537.53
	IsMobile: True

Desktop User-Agent: Mozilla/5.0 (Windows NT 6.3; WOW64; rv:41.0) Gecko/20100101 Firefox/41.0
	IsMobile: False

MediaHub User-Agent: Mozilla/5.0 (SMART-TV; Linux; Tizen 5.0) AppleWebKit/537.36 (KHTML, like Gecko) SamsungBrowser/2.1 TV Safari/537.36
	IsMobile: False
```

The IP Intelligence example looks up `185.28.167.77`,
`fdaa:bbcc:ddee:0:995f:d63a:f2a1:f189` and `127.0.0.1`. For each it logs, with
the date and time, `IP Address <address>:` and `Actual result:`, then a line
for each of `IpRangeStart`, `IpRangeEnd`, `AccuracyRadiusMin`,
`AccuracyRadiusMax`, `RegisteredCountry`, `RegisteredName`, `Longitude`,
`Latitude`, `Areas` and `Mcc`. The Lite file has RegisteredCountry,
RegisteredName and RegisteredOwner, so the lines for the properties it does not
have are empty, and `Mcc` reads `Mcc: :0.00`. After the first address it also
logs `Expected result:` and a block of text written in the source, because the
values differ from that text. The block does not mean the example failed. The
exit code is 0.

The paid data files have many more properties, as the
[pricing page](https://51degrees.com/pricing?utm_source=github&utm_medium=docs&utm_campaign=51degrees-go&utm_content=agents.md&utm_term=correct-output)
describes.

### Cloud examples

No example here calls the 51Degrees cloud service, so none needs a resource
key. The Go modules work on-premise, with a data file. The Pipeline API's
`examples/pipeline/4_cloud_engine` shows a cloud engine against a stand-in for
the service that the example starts itself, with the placeholder key
`example-resource-key`.

### Pitfalls

- **Running the Device Detection example before the download.** cxx still
  has an older data submodule, `cxx/device-detection-data`, whose copy of the
  file is a Git LFS pointer unless Git LFS has fetched it. The search in the
  published `dd` module, at the version `examples/device-detection/go.mod`
  requires (v4.5.169), does not pass over the pointer, although the one in
  `device-detection/dd/file.go` here does. So the example uses the pointer
  and stops with an error from `InitManagerFromFile`. Download the file into
  `cxx/data` first, which the search reaches before that folder, and check the
  file sizes as above.
- **Running from the wrong folder.** There is no `go.mod` at the repository
  root, and each examples folder is a module of its own, so run `go run` from
  that folder. Without `_51DEGREES_IPI_PATH`, the IP Intelligence example finds no data
  file wherever it runs, unless a paid file named
  `51Degrees-EnterpriseIpiV41.ipi` is in that folder.
- **cgo turned off or no C compiler.** The engines are C code built through
  cgo, so Go needs `CGO_ENABLED=1` and a C compiler on the `PATH`, as the
  READMEs say.
- **A linker warning on macOS.** The linker warns that `-lm` is given more
  than once. The warning is harmless (`ci/products.ps1`).
- **Memory.** The IP Intelligence example loads the whole data file into
  memory (`ipi_interop.InMemory`), so it needs about 0.5 GB free.
- **What CI does not test.** CI runs the IP Intelligence examples only with a
  paid data file (`ci/run-integration-tests.ps1`), never the Lite file, and
  runs the Device Detection examples from `examples/device-detection/dd`
  through `go test`, with the Lite file linked into `examples/device-detection`
  (`ci/fetch-assets.ps1`) rather than read from `cxx`.

## Module paths and tags

Each module's path is the repository path plus its folder, such as
`github.com/51Degrees/51degrees-go/device-detection/v4`. Go finds a version
of a module in a folder by a tag named after the folder, such as
`device-detection/v4.5.166`, so a release is those tags
(`ci/publish-package.ps1`), and the repository is also tagged with the plain
version. Never move or delete a released tag: sum.golang.org keeps the
checksum of the first version it sees, so a changed version breaks every build
that uses it.

The repository has one version, and only the patch number moves
(`ci/get-next-package-version.ps1`).

## The workspace

Each module requires the others at a published version, so the examples work
as a customer's code would when copied out. The committed `go.work` at the root
uses every module here instead, so a fresh clone builds every example against
the code in this repository with no settings. `ci/create-workspace.ps1` makes
it. Go still looks for a required version that isn't published even when that
module is in the workspace, so the script also adds a `replace` for each
requirement between these modules. Run the script after changing such a
requirement and commit `go.work`, because CI fails when the committed one
differs from what the script makes (`ci/setup-environment.ps1`). The nightly
package update runs the script as well. Nothing else may add a `replace` to a
`go.mod`, as it would be published with the module.

CI tests with the workspace when a folder of a published module (`common`,
`device-detection`, `ip-intelligence`, `pipeline`) has changed since the
latest release, that is when it tests the next release. Otherwise CI tests the
latest release, so it removes `go.work` from its own checkout and the examples
run against the published modules at that version. An example's `go.mod` can
name an earlier release, because the nightly package update moves the examples
to the latest release on `rc`, and `main` has the move only after its next
release. So CI moves those requirements first, in its own checkout, and
commits nothing. The same check decides whether there is a release at all
(`ci/get-next-package-version.ps1`), so a change to the examples, the
documentation or CI alone releases nothing.

An example's `go` line is at least the highest `go` line of the published
modules it requires, because Go won't take a module that asks for a newer Go
than the example does. The nightly package update raises it to that line at the
latest release (`Get-ReleaseGoVersion` in `ci/products.ps1`). When a published
module raises its `go` line, the minimum Go version under "Prerequisites" and
in that module's README moves with it.

CI also builds each published module without the workspace (`GOWORK=off`), to
check the module as it is published.

When an engine needs a change to `common` or `pipeline` in the same release,
its `go.mod` requires that module at the version the next release publishes,
because the release tags every module at the same commit. That version is not
published until then, so the build without the workspace takes it from the
module's folder through a copy of `go.mod`, and the nightly package update
leaves the module alone (`Get-UnreleasedRequirementReplaces` in
`ci/products.ps1`).

## The C sources

`device-detection/dd/device-detection-cxx.[ch]` and
`ip-intelligence/ipi_interop/ip-intelligence-cxx.[ch]` are generated from the
`cxx` submodule by `ci/update-amalgamation.ps1`. Never edit them by hand; the
build fails when they don't match `cxx`. `.gitattributes` keeps their line
endings as the C sources have them, so on Windows check `cxx` out with
`core.autocrlf` set to false before generating them.

IP Intelligence's copy of the common C library is renamed with the list in
`cxx/src/common/ipiSymbols.h` (`FIFTYONE_DEGREES_IPI_SYMBOLS` in
`ipi_interop/cgoflags.go`), so one program can use both engines. The mixed
examples check that.

## The JavaScript template

The JavaScript Builder Element renders
[javascript-templates](https://github.com/51Degrees/javascript-templates)'
`JavaScriptResource.mustache`, as every other 51Degrees language does. A Go
module carries no submodule, so
`pipeline/elements/javascriptbuilder/javascript-templates` holds a copy of the
`.mustache` files in the `javascript-templates` submodule. Never edit the copy
by hand. The JavaScript templates check (`ci/check-javascript-templates.ps1`)
fails when the copy differs from the submodule, and the nightly package update
moves the submodule and the copy together.

## Data files

The data files come from the [data](https://github.com/51Degrees/data)
repository, which is the `cxx/data` submodule. That repository holds a
`<file>.link` for each file, and its `get-data-files.ps1` and
`get-data-files.sh` scripts save the file beside the link. CI takes every file
from common-ci's assets instead, which each run caches once, and
`ci/fetch-assets.ps1` copies or links them into the folders the tests search,
so no test job downloads a data file. A file added to the data repository
needs an entry in common-ci's `steps/fetch-assets.ps1` before CI can use it.

When an example's data file is missing and the data repository links to it,
the examples' helpers run the script for that file
(`examples/device-detection/dd/example_base.go` and
`examples/ip-intelligence/common/file.go`). cxx still has the old
`device-detection-data` submodule, whose data files are Git LFS pointers
unless Git LFS has fetched them, so `dd.GetFilePath` passes over Git LFS
pointer files.

## CI

The workflows call the shared workflows in
[common-ci](https://github.com/51Degrees/common-ci), which run the scripts in
`ci/`. `ci/products.ps1` works out which modules a change affects.

The Pull Requests workflow is started by hand. A run that passes squash-merges
the pull request unless `dryrun` is ticked, which loses the pull request's own
commits. That workflow and the nightly pipeline merge only into `rc`, never
into `main`, because common-ci's `steps/get-target-branch.ps1` chooses `rc`
for a repository that has one. The nightly pipeline merges every pull request
into `rc` that passes, is not a draft and has no review requested, and
publishes `main` when it has changed.

## Releases

Pull requests go to `rc`, not `main`, and stay drafts until they have been
reviewed, because the nightly merges any pull request into `rc` that is not a
draft and has no review requested. When a release is wanted, one pull request
merges `rc` into `main`, and the "Only rc into main" check fails a pull request
into `main` from any other branch.

## History

The history of common-go, device-detection-go, ip-intelligence-go and the Go
pipeline library was brought in with `git subtree add`, so commit SHAs match the
original repositories. Use `git log --follow` to trace a file back through its move.

## Tests that fail on timing

A test that fails on a time limit, a performance threshold or a timeout is
fixed, not run again until it passes. Either the limit is set to fit the
slowest runner the test runs on, with each runner's measurement behind it, or
the reason it takes too long is found and fixed. A failure that comes and goes
costs everyone who meets it the time to look into it.
