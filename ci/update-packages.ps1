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

# Update the dependencies of each module, without the workspace. The examples
# move to the latest release of the published modules. A change to them alone
# doesn't make a release. An example that requires a module not released yet
# is left as it is until that release is out, and so is a published module
# that requires the version of another the next release publishes.
. $PSScriptRoot/products.ps1
$repoRoot = (Resolve-Path $RepoName).Path
foreach ($module in 'common', 'device-detection', 'ip-intelligence', 'pipeline',
    'examples/device-detection', 'examples/ip-intelligence', 'examples/mixed',
    'examples/pipeline') {
    if ($module -notlike 'examples/*' -and
        @(Get-UnreleasedRequirementReplaces $repoRoot $module)) {
        Write-Warning "$module requires a module at the version the next release publishes, so it is left as it is"
        continue
    }
    Push-Location (Join-Path $RepoName $module)
    try {
        $env:GOWORK = 'off'
        if ($module -like 'examples/*') {
            # tidy, not go list, which also fails when go.sum only lacks the
            # lines for a release, as it does when an example is first moved
            # from the workspace to a published module.
            $PSNativeCommandUseErrorActionPreference = $false
            go mod tidy *> $null
            $resolved = $LASTEXITCODE -eq 0
            $PSNativeCommandUseErrorActionPreference = $true
            if (!$resolved) {
                $global:LASTEXITCODE = 0
                Write-Warning "$module requires a module that isn't released yet, so it is left as it is"
                continue
            }
        }
        # The go line stays as it is, so an update never asks for a newer Go
        # than CI tests with. When the latest versions would, only patch
        # versions are taken.
        $goVersion = (go mod edit -json | ConvertFrom-Json).Go
        Write-Output "Updating the dependencies of $module"
        $PSNativeCommandUseErrorActionPreference = $false
        go get -u ./... "go@$goVersion" toolchain@none
        $updated = $LASTEXITCODE -eq 0
        $PSNativeCommandUseErrorActionPreference = $true
        if (!$updated) {
            $global:LASTEXITCODE = 0
            Write-Warning "The latest dependencies of $module need a newer Go than $goVersion, so only patch versions are taken"
            go get -u=patch ./... "go@$goVersion" toolchain@none
        }
        # go@1.22 means the latest 1.22 release, so the line is set back.
        go mod edit "-go=$goVersion" -toolchain=none
        go mod tidy
    } finally {
        Remove-Item Env:GOWORK
        Pop-Location
    }
}
