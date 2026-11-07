param (
    [Parameter(Mandatory)][string]$RepoName,
    [string]$Repository,
    [string]$HowToAllow
)
$ErrorActionPreference = "Stop"
. (Join-Path $PSScriptRoot 'packages.ps1')

# The shared publish runs this in place of its own check, because each module
# has its own version and releases (see packages.ps1). Every module about to be
# published goes through common-ci's steps/check-release-interval.ps1, run from
# the common-ci root, against its own tags, which refuses a release less than a
# week after that module's last unless ALLOW_EARLY_RELEASE is 'true'.
$plan = @(Get-ModuleReleasePlan $RepoName)
if ($plan.Count -eq 0) {
    Write-Output "No module is to be published, so there is no release to check."
    exit 0
}

$arguments = @{ RepoName = $RepoName }
if ($Repository) {
    $arguments.Repository = $Repository
}
if ($HowToAllow) {
    $arguments.HowToAllow = $HowToAllow
}

$refused = @()
foreach ($release in $plan) {
    Write-Output "$($release.Name) $($release.Version):"
    $global:LASTEXITCODE = 0
    ./steps/check-release-interval.ps1 @arguments -TagPrefix $release.Path
    if ($LASTEXITCODE -ne 0) {
        $refused += $release.Name
    }
}

if ($refused) {
    Write-Output "Nothing is published, because these modules were released less than a week ago: $($refused -join ', ')."
    exit 1
}
exit 0
