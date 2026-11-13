# Shared by the scripts in this folder, which dot-source it. Describes the Go
# modules this repository builds and tests, and works out which of them a pull
# request touches.

$AllProducts = @('common', 'device-detection', 'ip-intelligence', 'pipeline')

# Folders that belong to a single product. A change anywhere else, including
# common, which both engines are built on, and CI, affects every product.
$ProductPaths = @{
    'device-detection' = @('device-detection/', 'examples/device-detection/')
    'ip-intelligence' = @('ip-intelligence/', 'examples/ip-intelligence/')
    'pipeline' = @('pipeline/', 'examples/pipeline/')
}

# Returns the products the changed paths affect.
function Resolve-AffectedProducts([string[]]$Changed) {
    if (!$Changed) { return $AllProducts }
    $affected = [System.Collections.Generic.List[string]]::new()
    foreach ($path in $Changed) {
        $owner = $null
        foreach ($product in $ProductPaths.Keys) {
            foreach ($prefix in $ProductPaths[$product]) {
                if ($path.StartsWith($prefix)) { $owner = $product }
            }
        }
        if (!$owner) { return $AllProducts }
        if (!$affected.Contains($owner)) { $affected.Add($owner) }
    }
    return $affected.ToArray()
}

# Returns the products a pull request changes, or every product when the
# checkout is not a pull request (a publish, or a build of the branch itself).
# A pull request is checked out with the branch it merges into merged in, so
# the merge base with that branch leaves only the pull request's own changes.
function Get-AffectedProducts([string]$RepoPath, [string]$Branch = 'main') {
    $head = git -C $RepoPath rev-parse HEAD
    $base = git -C $RepoPath merge-base "origin/$Branch" HEAD 2>$null
    if ($LASTEXITCODE -ne 0 -or !$base -or $base -eq $head) {
        $global:LASTEXITCODE = 0
        return $AllProducts
    }
    $products = Resolve-AffectedProducts (git -C $RepoPath diff --name-only $base $head)
    $skipped = @($AllProducts | Where-Object { $products -notcontains $_ })
    if ($skipped) {
        Write-Host "Not affected by this change, so skipped: $($skipped -join ', ')"
    }
    return $products
}

# The folder of a product's Go module.
function Get-ProductPath([string]$RepoPath, [string]$Product) {
    return Join-Path $RepoPath $Product
}

# Runs go test with the given arguments in a module folder, writing JUnit
# results. Returns the exit code.
function Invoke-GoTests {
    param(
        [Parameter(Mandatory)][string]$ModulePath,
        [Parameter(Mandatory)][string[]]$Packages,
        [Parameter(Mandatory)][string]$JUnitPath
    )
    # Absolute, because the tests run from the module folder.
    $JUnitPath = [System.IO.Path]::GetFullPath($JUnitPath)
    New-Item -ItemType Directory -Force -Path (Split-Path $JUnitPath) | Out-Null
    if ($IsMacOS) {
        # go-junit-report counts the harmless linker warning about -lm being
        # given more than once as an error.
        $env:CGO_LDFLAGS = '-Wl,-no_warn_duplicate_libraries'
    }
    Push-Location $ModulePath
    try {
        Write-Host "Testing $Packages in '$ModulePath'"
        go test -v @Packages 2>&1 | go-junit-report -set-exit-code -iocopy -out $JUnitPath | Out-Host
        return $LASTEXITCODE
    } finally {
        Pop-Location
    }
}

# The folders of the published modules. A release is needed only when one of
# them has changed.
$LibraryPaths = @('common', 'device-detection', 'ip-intelligence', 'pipeline')

# Returns the latest release, the highest plain version tag, or nothing.
function Get-LatestRelease([string]$RepoPath) {
    git -C $RepoPath tag --list | Where-Object { $_ -match '^\d+\.\d+\.\d+$' } |
        Sort-Object { [version]$_ } | Select-Object -Last 1
}

# Returns whether a published module has changed since the latest release,
# which is what a new release would publish.
function Test-UnreleasedLibraryChange([string]$RepoPath) {
    $release = Get-LatestRelease $RepoPath
    if (!$release) { return $true }
    $changed = git -C $RepoPath diff --name-only $release HEAD -- @LibraryPaths
    return [bool]$changed
}

# Returns a go line's version in a form that compares correctly, as 1.22 and
# 1.22.0 name the same release.
function ConvertTo-GoVersion([string]$Version) {
    $parts = @($Version.Split('.'))
    while ($parts.Count -lt 3) { $parts += '0' }
    [version]($parts[0..2] -join '.')
}

# Returns the highest go line of the published modules that the module in the
# folder requires, as they are at the latest release, or nothing.
function Get-ReleaseGoVersion([string]$RepoPath, [string]$ModuleFolder) {
    $release = Get-LatestRelease $RepoPath
    if (!$release) { return }
    $mod = go mod edit -json (Join-Path $RepoPath $ModuleFolder 'go.mod') | ConvertFrom-Json
    $versions = foreach ($require in $mod.Require) {
        foreach ($library in $LibraryPaths) {
            if ($require.Path -like "github.com/51Degrees/51degrees-go/$library/v*") {
                $line = git -C $RepoPath show "${release}:$library/go.mod" |
                    Select-String '^go (\S+)'
                $line.Matches[0].Groups[1].Value
            }
        }
    }
    $versions | Sort-Object { ConvertTo-GoVersion $_ } | Select-Object -Last 1
}

# Returns a go mod edit -replace argument for each requirement of the module in
# the folder on a module of this repository at a version not released yet. Such
# a version is the one the next release publishes, from the same commit, so
# the module's folder here is what that version will hold.
function Get-UnreleasedRequirementReplaces([string]$RepoPath, [string]$ModuleFolder) {
    $mod = go mod edit -json (Join-Path $RepoPath $ModuleFolder 'go.mod') | ConvertFrom-Json
    foreach ($require in $mod.Require) {
        foreach ($library in $LibraryPaths) {
            if ($require.Path -like "github.com/51Degrees/51degrees-go/$library/v*" -and
                !(git -C $RepoPath tag --list "$library/$($require.Version)")) {
                $folder = (Resolve-Path (Join-Path $RepoPath $library)).Path
                "-replace=$($require.Path)@$($require.Version)=$folder"
            }
        }
    }
}
