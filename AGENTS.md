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
So `go run` downloads those modules and compiles their C code through cgo, and
nothing in this repository needs building first. `pwsh ci/create-workspace.ps1`
makes the examples use the code in this repository instead, as the README says.

### Prerequisites

- Git, and [Git LFS](https://git-lfs.github.com/) for the Device Detection
  data file.
- Go 1.21 or later, because the IP Intelligence modules ask for 1.21.0. CI
  tests Go 1.23 and 1.24 (`ci/options.json`).
- A C compiler for cgo, being GCC on Linux, Clang on macOS and MinGW-x64 on
  Windows, with `libatomic`, which usually comes with GCC and Clang.
- On Windows, the MinGW-x64 `bin` folder on the `PATH` (by default
  `C:\msys64\ucrt64\bin`) and cgo turned on, as the examples' READMEs say.

  ```
  go env -w CGO_ENABLED=1
  ```

- Network access to GitHub, and to the Go module proxy for the published
  modules.
- For the IP Intelligence data file, PowerShell 7 (`pwsh`) on any system, or
  `curl`, `gunzip` and `md5sum` (`md5` on macOS) for the shell script on Linux
  and macOS.

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

The examples use the free Lite data files, which need no licence key. They
come from the `cxx` submodule, and two layouts exist. Read the
"Working on the code" section of `README.md` in your checkout to find out
which one applies.

- **The section gives no command for the data files.** This is the layout on
  `rc` and `main` today. The files are in `cxx/device-detection-data` and
  `cxx/ip-intelligence-data`. Follow "Without cxx/data".
- **The section gives `sh cxx/data/get-data-files.sh`.** The draft pull request
  [#41](https://github.com/51Degrees/51degrees-go/pull/41) moves the examples
  to the [data](https://github.com/51Degrees/data) repository, which cxx
  carries as `cxx/data`. Once it is merged, follow "With cxx/data".

Go by the README rather than by whether `cxx/data` exists, because the folder
can arrive with a newer `cxx` before the examples here read it.

Do not use `pwsh ci/fetch-assets.ps1 .`, which the module and example READMEs
give. It calls common-ci's `steps/fetch-assets.ps1` from the folder it runs
in, so it works only inside CI.

#### Without cxx/data

The Device Detection Lite file is stored with Git LFS. Install Git LFS, for
example with `sudo apt-get install git-lfs` on Ubuntu, then pull the file in
its submodule, as `cxx/README.md` says.

```
git lfs install
cd cxx/device-detection-data
git lfs pull
cd ../..
```

A script in `cxx/ip-intelligence-data` downloads the IP Intelligence Lite
file. Run it from inside that folder, because it saves the file in the folder
it runs from.

```
cd cxx/ip-intelligence-data
pwsh ./get-lite-file-from-azure.ps1
cd ../..
```

On Linux and macOS, `./get-lite-file-from-azure.sh` in the same folder does
the same. Either script also downloads the Asn file over the copy committed in
the submodule, so `git status` then shows the submodule as modified, which is
expected.

#### With cxx/data

```
sh cxx/data/get-data-files.sh
```

In PowerShell, run `./cxx/data/get-data-files.ps1` instead. Either script
downloads every free file into `cxx/data/device-detection` and
`cxx/data/ip-intelligence`, and keeps a file that is already there. In this
layout the `dd` and `onpremise` examples, and the IP Intelligence examples
given a `DATA_FILE` that the data repository links to, download a missing file
themselves.

#### Check the files

- `cxx/device-detection-data/51Degrees-LiteV4.1.hash` is more than 10 MB. A
  file of about 130 bytes that starts with
  `version https://git-lfs.github.com/spec/v1` is a Git LFS pointer, which
  means Git LFS has not pulled the file.
- `cxx/ip-intelligence-data/51Degrees-LiteV41.ipi` is more than 100 MB.

With `cxx/data`, the same files are in `cxx/data/device-detection` and
`cxx/data/ip-intelligence`.

### Run

Run each example from its own module folder, because there is no `go.mod` at
the repository root. The first run downloads the modules and compiles the C
library, so it takes longer than later runs.

The Device Detection example finds `51Degrees-LiteV4.1.hash` by searching
every folder below the folder two above the one it runs in
(`GetFilePathByName` in `examples/device-detection/dd/example_base.go`). From
`examples/device-detection` that is the repository root, so the search reaches
the file in `cxx`.

```
cd examples/device-detection
go run ./dd/getting_started
cd ../..
```

The IP Intelligence example reads the data file named by the `DATA_FILE`
environment variable, and otherwise looks for `51Degrees-EnterpriseIpiV41.ipi`
in the folder it runs in, without searching
(`examples/ip-intelligence/common/example_base.go`). So name the Lite file.

```
cd examples/ip-intelligence
DATA_FILE=../../cxx/ip-intelligence-data/51Degrees-LiteV41.ipi go run ./getting_started
cd ../..
```

In PowerShell, set the variable first.

```
cd examples/ip-intelligence
$env:DATA_FILE = '../../cxx/ip-intelligence-data/51Degrees-LiteV41.ipi'
go run ./getting_started
cd ../..
```

With `cxx/data`, the path is
`../../cxx/data/ip-intelligence/51Degrees-LiteV41.ipi`.

### Correct output

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

- **A Git LFS pointer instead of the data file.** The Device Detection search
  takes the first file with the right name, and on a checkout without Git LFS
  that is the pointer in `cxx/device-detection-data`. The example then stops
  with an error from `InitManagerFromFile`. Check the file sizes as above.
- **Running from the wrong folder.** There is no `go.mod` at the repository
  root, and each examples folder is a module of its own, so run `go run` from
  that folder. Without `DATA_FILE`, the IP Intelligence example finds no data
  file wherever it runs, unless a paid file named
  `51Degrees-EnterpriseIpiV41.ipi` is in that folder.
- **cgo turned off or no C compiler.** The engines are C code built through
  cgo, so Go needs `CGO_ENABLED=1` and a C compiler on the `PATH`, as the
  READMEs say.
- **Windows line endings.** Git for Windows usually checks files out with
  `core.autocrlf=true`, and `cxx/ip-intelligence-data` has no
  `.gitattributes`, so its shell script gets Windows line endings and the
  shell cannot run it. Use the PowerShell script on Windows.
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
as a customer's code would when copied out. `ci/create-workspace.ps1` makes a
`go.work` that uses every module here instead. Go still looks for a required
version that isn't published even when that module is in the workspace, so the
script also adds a `replace` for each requirement between these modules.
`go.work` is never committed, and nothing else may add a `replace` to a
`go.mod`, as it would be published with the module.

CI makes the workspace only when a folder of a published module (`common`,
`device-detection`, `ip-intelligence`, `pipeline`) has changed since the
latest release, that is when it tests the next release
(`ci/setup-environment.ps1`). The examples then run against that code, as the
.NET repository's CI does with its projects. Otherwise they run against the published modules their go.mod files
name. The same check decides whether there is a release at all
(`ci/get-next-package-version.ps1`), so a change to the examples, the
documentation or CI alone releases nothing. The nightly package update moves
the examples to the latest release.

CI also builds each published module without the workspace (`GOWORK=off`), to
check the module as it is published.

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

## Data files

The Lite data file in `cxx/device-detection-data` is a Git LFS file, which CI
doesn't fetch. Code that searches folders for `51Degrees-LiteV4.1.hash` can
find that pointer first, so give tools the fetched file explicitly, as
`ci/run-performance-tests.ps1` does. `ci/fetch-assets.ps1` links the data files
into the folders the tests search.

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
