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
environment variable, and otherwise the free Lite file `51Degrees-LiteV41.ipi`
(`LiteDataFile` in `examples/ip-intelligence/common/example_base.go`). It uses
that file where it runs when the file is there, and otherwise the data
repository's copy in `cxx/data/ip-intelligence`, which it downloads first when
it is missing (`GetDataFile` in `examples/ip-intelligence/common/file.go`).

```
cd examples/ip-intelligence
go run ./getting_started
cd ../..
```

### Correct output

When an example downloads its data file, it first logs
`<file> is missing, so it is being downloaded by running` with the command on
the next line, and the script's `Downloading` and `Saved` lines follow.

The Device Detection example prints these lines with the Lite file. The
properties only the Enterprise data file has are shown as needing it, with a
link to ask for a copy, and the exit code is 0. CI runs it with the Lite file,
and its test checks those lines (`CheckEnterprise` in
`examples/device-detection/dd/enterprise_check.go`).

```
Mobile User-Agent: Mozilla/5.0 (iPhone; CPU iPhone OS 7_1 like Mac OS X) AppleWebKit/537.51.2 (KHTML, like Gecko) Version/7.0 Mobile/11D167 Safari/9537.53
	IsMobile: True
	PlatformName: iOS
	PlatformVersion: 7.1
	BrowserName: Mobile Safari
	BrowserVersion: 7.0
	HardwareVendor: needs the Enterprise data file
	HardwareName: needs the Enterprise data file
	DeviceType: needs the Enterprise data file

Desktop User-Agent: Mozilla/5.0 (Windows NT 6.3; WOW64; rv:41.0) Gecko/20100101 Firefox/41.0
	IsMobile: False
	PlatformName: Windows
	PlatformVersion: 8.1
	BrowserName: Firefox
	BrowserVersion: 41.0
	HardwareVendor: needs the Enterprise data file
	HardwareName: needs the Enterprise data file
	DeviceType: needs the Enterprise data file

MediaHub User-Agent: Mozilla/5.0 (SMART-TV; Linux; Tizen 5.0) AppleWebKit/537.36 (KHTML, like Gecko) SamsungBrowser/2.1 TV Safari/537.36
	IsMobile: False
	PlatformName: Tizen
	PlatformVersion: 5.0
	BrowserName: Samsung Browser
	BrowserVersion: 9.2
	HardwareVendor: needs the Enterprise data file
	HardwareName: needs the Enterprise data file
	DeviceType: needs the Enterprise data file

Some of these properties need the Enterprise data file. Contact us for a copy at <the contact us link>
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

One example calls the 51Degrees cloud service,
`examples/ip-intelligence/getting_started_cloud`, which needs a Resource Key in
the `_51DEGREES_RESOURCE_KEY` environment variable or as its first argument,
and stops with a message saying where to create one when it has none
(`main.go` lines 113 to 123). The other examples work on-premise, with a data
file. The Pipeline API's `examples/pipeline/4_cloud_engine` shows a cloud
engine against a stand-in for the service that the example starts itself,
with the placeholder key `example-resource-key`.

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
  that folder.
- **cgo turned off or no C compiler.** The engines are C code built through
  cgo, so Go needs `CGO_ENABLED=1` and a C compiler on the `PATH`, as the
  READMEs say.
- **A linker warning on macOS.** The linker warns that `-lm` is given more
  than once. The warning is harmless (`ci/products.ps1`).
- **Memory.** The IP Intelligence example loads the whole data file into
  memory (`ipi_interop.InMemory`), so it needs about 0.5 GB free.
- **What CI runs.** CI runs the IP Intelligence examples with the paid data
  file `ci/run-integration-tests.ps1` names, and again with no settings on the
  free files (the `FreeDataFile` tests in
  `examples/ip-intelligence/integration_test.go`). It runs the Device Detection
  examples through `go test` in `examples/device-detection`, with the Lite
  file linked into that folder (`ci/fetch-assets.ps1`) rather than read from
  `cxx`, and the on-premise ones again with no settings (the `FreeDataFile`
  tests in `examples/device-detection/onpremise/onpremise_test.go`).

## Environment variable names

[common-ci's list of environment variable names](https://github.com/51Degrees/common-ci/blob/18f9a570ed862a0b7e1ecb1c55a8319f35f02491/environment-variables.md)
holds every environment variable name that the packages, tests, examples and
CI scripts of every language read, with what each name holds and the
deprecated names each replaced. The list decides. Use the names it gives, and
when a change needs a new name, or reads an old name again, change
[`environment-variables.json`](https://github.com/51Degrees/common-ci/blob/18f9a570ed862a0b7e1ecb1c55a8319f35f02491/environment-variables.json)
in common-ci in the same piece of work. This repository's own name
check, [`common/variable_names_test.go`](common/variable_names_test.go), still keeps a
list of its own. Where the two differ, common-ci's list is the one to
follow.

## Module paths and tags

Each module's path is the repository path plus its folder, such as
`github.com/51Degrees/51degrees-go/device-detection/v4`. Go finds a version
of a module in a folder by a tag named after the folder, such as
`device-detection/v4.5.172`, so a release is those tags
(`ci/publish-package.ps1`), and each tag gets a GitHub release.
Never move or delete a released tag, because sum.golang.org keeps the checksum
of the first version it sees, and a changed version breaks every build that
uses it.

Each module has its own version, and only the patch number moves. A publish
releases a module when a file in its folder has changed since its last tag,
and every module that requires it is released with it. `ci/packages.json`
lists the modules and which of them each requires, which has to match the
`go.mod` files, and common-ci's `steps/release-plan.ps1` works out the plan
from it (`ci/packages.ps1`). A change to the examples, to CI or to the
documentation outside the module folders releases nothing, and neither does a
change inside them only to tests (`*_test.go`), their data (`mock_hash.gz`),
Markdown files, `docs` folders or licences. Run the tests of the plan with
`Invoke-Pester ci/packages.Tests.ps1`, with `COMMON_CI` naming a checkout of
common-ci. The CI script tests workflow runs these and the build selection
tests on every pull request that changes `ci/`.

A module that requires one released in the same publish is tagged on a commit
that raises its `go.mod` to the new version, because Go reads the requirement
at the tag. Only the tag reaches that commit, and the nightly package update
makes the same change on `rc`.

common-ci's release plan reads tags named `<package>/<version>`, without Go's
`v`, so `ci/packages.ps1` gives each module's newest tag a local tag of that
form before planning. Those local tags are never pushed.

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

Go's [modules reference](https://go.dev/ref/mod#workspaces) calls committing
`go.work` "generally inadvisable", and allows it for modules "developed
exclusively with each other" when "the module author should ensure the
individual modules are tested and released properly". These modules are
developed only with each other, so CI does that in two ways. It builds each
published module without the workspace (`GOWORK=off` in
`ci/build-project.ps1`), and after each release it tests the examples against
the published modules with `go.work` removed (`ci/setup-environment.ps1`).

CI tests with the workspace when the release plan has a module in it, that is
when a published module (`common`, `device-detection`, `ip-intelligence`,
`pipeline`) has changed since its last release and CI tests the next release.
Otherwise CI tests the latest releases, so it removes `go.work` from its own
checkout and the examples run against the published modules, each at its
latest release. An example's `go.mod` can name an earlier release, because the
nightly package update moves the examples to the latest releases on `rc`, and
`main` has the move only after its next release. So CI moves those
requirements first, in its own checkout, and commits nothing. The same plan
decides what a publish releases (`ci/package-update-required.ps1`), so a
change to the examples, the documentation or CI alone releases nothing.

An example's `go` line is at least the highest `go` line of the published
modules it requires, because Go won't take a module that asks for a newer Go
than the example does. The nightly package update raises it to that line at
each module's latest release (`Get-ReleaseGoVersion` in `ci/products.ps1`).
When a published module raises its `go` line, the minimum Go version under
"Prerequisites" and in that module's README moves with it.

CI also builds each published module without the workspace (`GOWORK=off`), to
check the module as it is published.

When an engine needs a change to `common` or `pipeline` in the same release,
its `go.mod` requires that module at the version the release plan gives it,
the module's last version with one more patch. Any other version with no tag
stops the plan (`Assert-RequiredVersions` in `ci/packages.ps1`), and the
publish tags the engine on a commit whose `go.sum` matches. That version is not
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
`ci/`. `ci/products.ps1` works out which modules a change affects, measured
from the branch the pull request merges into, `rc`, and adds each module that
requires one of them (`DependsOn` in `ci/packages.json`), because the workspace
builds a module against the folders of the modules it requires.
`ci/select-options.ps1` leaves macOS and performance to the builds of `rc`
itself and of the release pull request, which
`.github/workflows/release-pull-request.yml` builds on every entry. Run the
tests of both with `Invoke-Pester ci/select-options.Tests.ps1`, with
`COMMON_CI` naming a checkout of common-ci.

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
