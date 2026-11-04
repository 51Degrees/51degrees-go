# Working in this repository

Notes on what is not obvious from the code. Keep them short and current.
The layout, the workspace and the C sources are described in the
[README](README.md).

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

The data files come from the [data](https://github.com/51Degrees/data)
repository, which is the `cxx/data` submodule. That repository holds a
`<file>.link` for each file, and its `get-data-files.ps1` and
`get-data-files.sh` scripts save the file beside the link. CI takes most files
from common-ci's assets instead, and `ci/fetch-assets.ps1` links them into the
folders the tests search. `ci/fetch-assets.ps1` also runs the data
repository's script to download the Asn file that the IP Intelligence reload
tests use.

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
commits. The nightly pipeline also merges every non-draft pull request into
`main` that passes, and publishes `main` when it has changed.

## History

The history of common-go, device-detection-go, ip-intelligence-go and the Go
pipeline library was brought in with `git subtree add`, so commit SHAs match the
original repositories. Use `git log --follow` to trace a file back through its move.
