# API Specific CI/CD Approach
This API complies with the `common-ci` approach defined [here](https://github.com/51Degrees/common-ci), with the following exceptions:

The following secrets are required:
* `ACCESS_TOKEN` - GitHub [access token](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens#about-personal-access-tokens) for cloning repos, creating PRs, etc.
    * Example: `github_pat_l0ng_r4nd0m_s7r1ng`

Unlike the compiled-and-published reference implementations, this Go module is
distributed purely as source via Git tags, so no package-manager API key or code
signing secrets are required.

## General
This is a pure Go module. The CI steps are implemented in PowerShell so they run
consistently across Windows, Linux and macOS, matching the `common-ci`
convention. Each step script accepts the build `Options` (from `options.json`)
as parameters.

The build matrix in [`options.json`](options.json) runs the unit tests on
Windows, Linux and macOS using the Go version recorded in `go.mod`.

## Build
`build-project.ps1` runs `go build ./...` and `go vet ./...` so compilation and
vet problems surface as a distinct, early failure. The tests build the packages
on demand, so no separately produced artifact is needed.

## Unit Tests
`run-unit-tests.ps1` runs `go test ./...` and pipes the output through
`go-junit-report` to produce `test-results/unit/results.xml` for the common-ci
test-results publishing step.

## Integration and Performance Tests
The generic Pipeline framework has no separate integration or performance suite:
the cloud engine and web integration are covered by the unit tests using
in-process `httptest` servers, and performance is measured in the engine
repositories (for example device detection) that depend on this one. The
corresponding scripts are present but are intentionally no-ops.
