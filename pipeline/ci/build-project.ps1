# Builds and vets the module.
#
# The unit tests build the packages on demand, so a separate build step is not
# strictly required. It is run here anyway so that compilation and `go vet`
# problems are surfaced as a distinct, early CI failure rather than as part of
# the test run.

param (
    [Parameter(Mandatory)][string]$RepoName
)
$ErrorActionPreference = "Stop"
$PSNativeCommandUseErrorActionPreference = $true

Push-Location $RepoName
try {
    Write-Output "Building all packages..."
    go build ./... || $(throw "build failed")

    Write-Output "Vetting all packages..."
    go vet ./... || $(throw "go vet failed")
} finally {
    Pop-Location
}
