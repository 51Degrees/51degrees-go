param (
    [Parameter(Mandatory)][string]$RepoName,
    [string]$Version
)
$ErrorActionPreference = "Stop"
. (Join-Path $PSScriptRoot 'packages.ps1')

# Called by common-ci's publish in place of its own check, which looks for a
# tag of the run's one version and cannot tell the modules apart. Exits 0 when
# the release plan has something in it and 1 when it is empty, as common-ci's
# check does. $Version is the run's label and is not used.
$plan = @(Get-ModuleReleasePlan $RepoName)
if ($plan.Count -eq 0) {
    Write-Output "No module has changed since its last release, so there is nothing to publish"
    exit 1
}

$packages = Get-Packages $RepoName
Write-Output "This publish releases, in this order:"
$plan | Format-Table Name, Previous, Version, @{
        Name = 'Tag'
        Expression = {
            $name = $_.Name
            Get-GoTag ($packages | Where-Object Name -eq $name) $_.Version
        }
    }, Reason -AutoSize | Out-String | Write-Output
exit 0
