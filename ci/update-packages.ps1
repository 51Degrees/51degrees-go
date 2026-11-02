param (
    [Parameter(Mandatory)][string]$RepoName,
    [Parameter(Mandatory)][string]$OrgName,
    [bool]$DryRun = $false
)
$ErrorActionPreference = "Stop"
$PSNativeCommandUseErrorActionPreference = $true

$cxx = Join-Path $RepoName cxx

# Move cxx to the latest 51degrees-cxx release, and make the engines' C sources
# from it.
git -C $cxx fetch --quiet --tags origin
$release = git -C $cxx tag --list | Where-Object { $_ -match '^\d+\.\d+\.\d+$' } |
    Sort-Object { [version]$_ } | Select-Object -Last 1
$current = git -C $cxx describe --tags --always
Write-Output "cxx is at $current, the latest release is $release"
if ($release -and $release -ne $current) {
    git -C $cxx checkout --quiet $release
    git -C $RepoName submodule update --init --recursive cxx
}
& (Join-Path $RepoName ci/update-amalgamation.ps1)

# Update the dependencies of the published modules. The examples require the
# modules at their published version, which a release moves.
foreach ($module in 'common', 'device-detection', 'ip-intelligence') {
    Push-Location (Join-Path $RepoName $module)
    try {
        $env:GOWORK = 'off'
        Write-Output "Updating the dependencies of $module"
        go get -u ./...
        go mod tidy
    } finally {
        Remove-Item Env:GOWORK
        Pop-Location
    }
}
