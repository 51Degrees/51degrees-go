# Determines the next package (module) version using GitVersion, storing it in
# the named variable for the calling publish workflow. Go modules are versioned
# by Git tags, so this version is used to tag the release.

param (
    [Parameter(Mandatory)][string]$RepoName,
    [string]$VariableName = "Version"
)
$ErrorActionPreference = "Stop"

Set-Variable -Scope Global -Name $VariableName -Value "v$(./steps/get-next-package-version.ps1 -RepoName $RepoName)"
