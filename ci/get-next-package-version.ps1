param (
    [Parameter(Mandatory)][string]$RepoName,
    [Parameter(Mandatory)][string]$VariableName
)
$ErrorActionPreference = "Stop"
. (Join-Path $PSScriptRoot 'packages.ps1')

# common-ci's publish carries one version through its jobs. Each module here
# has its own version (see packages.ps1), so that one version is only a label
# for the run, the highest version this publish releases. Whether anything is
# released is decided by package-update-required.ps1, and each module's tag by
# publish-package.ps1, both from the same release plan.
$plan = @(Get-ModuleReleasePlan $RepoName)
if ($plan.Count -gt 0) {
    # @() keeps a plan of one module a list, as indexing a lone string would
    # give its last character.
    $label = @($plan.Version | Sort-Object { [version]$_ })[-1]
} else {
    # Nothing to release, so the label is the newest version already
    # released, and package-update-required.ps1 stops the publish.
    $released = foreach ($package in Get-Packages $RepoName) {
        @(Get-ReleasedVersions $RepoName $package.Name) | Select-Object -Last 1
    }
    $label = "$(@($released | Sort-Object) | Select-Object -Last 1)"
}
Write-Output "The label for this publish is $label"

Set-Variable -Scope Global -Name $VariableName -Value $label
