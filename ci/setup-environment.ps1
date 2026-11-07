$ErrorActionPreference = "Stop"

./go/setup-environment.ps1

. $PSScriptRoot/packages.ps1

# When a published module has changed since its last release, the tests are
# of the next release, so the examples use the modules in this repository.
# Otherwise they use the published modules their go.mod files name, as a
# customer's code does.
$repo = Join-Path $PSScriptRoot ".."
Remove-Item (Join-Path $repo go.work), (Join-Path $repo go.work.sum) -ErrorAction SilentlyContinue
$plan = @(Get-ModuleReleasePlan $repo)
if ($plan.Count -gt 0) {
    Write-Host "$($plan.Name -join ', ') would be released, so the examples use the modules in this repository"
    & $PSScriptRoot/create-workspace.ps1
} else {
    Write-Host "No published module has changed since its last release, so the examples use the published modules"
}
