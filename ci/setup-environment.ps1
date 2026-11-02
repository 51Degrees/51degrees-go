$ErrorActionPreference = "Stop"

./go/setup-environment.ps1

. $PSScriptRoot/products.ps1

# When a published module has changed since the latest release, the tests are
# of the next release, so the examples use the modules in this repository.
# Otherwise they use the published modules their go.mod files name, as a
# customer's code does.
$repo = Join-Path $PSScriptRoot ".."
Remove-Item (Join-Path $repo go.work), (Join-Path $repo go.work.sum) -ErrorAction SilentlyContinue
if (Test-UnreleasedLibraryChange $repo) {
    & $PSScriptRoot/create-workspace.ps1
} else {
    Write-Host "No published module has changed since $(Get-LatestRelease $repo), so the examples use the published modules"
}
