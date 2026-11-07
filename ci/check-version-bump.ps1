param (
    [Parameter(Mandatory)][string]$RepoName,
    [string]$Version
)
$ErrorActionPreference = "Stop"
. (Join-Path $PSScriptRoot 'packages.ps1')

# The shared publish runs this in place of its own check, because each module
# has its own version (see packages.ps1). Every module about to be published
# goes through common-ci's steps/check-version-bump.ps1, run from the common-ci
# root, against its own tags, which refuses a major or minor move unless
# ALLOW_VERSION_BUMP is 'true'. $Version is the run's label, the highest of
# the modules' versions.
$plan = @(Get-ModuleReleasePlan $RepoName)
if ($plan.Count -eq 0) {
    Write-Output "No module is to be published, so there is no version to check."
    exit 0
}

$refused = @()
foreach ($release in $plan) {
    Write-Output "$($release.Name) $($release.Version):"
    $global:LASTEXITCODE = 0
    ./steps/check-version-bump.ps1 -Version $release.Version -RepoName $RepoName `
        -TagPrefix $release.Path
    if ($LASTEXITCODE -ne 0) {
        $refused += $release.Name
    }
}

if ($refused) {
    Write-Output "Nothing is published, because the versions of these modules were refused: $($refused -join ', ')."
    exit 1
}
exit 0
