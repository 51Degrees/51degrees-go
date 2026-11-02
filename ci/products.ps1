# Shared by the scripts in this folder, which dot-source it. Describes the Go
# modules this repository builds and tests, and works out which of them a pull
# request touches.

$AllProducts = @('common', 'device-detection', 'ip-intelligence')

# Folders that belong to a single product. A change anywhere else, including
# common, which both engines are built on, and CI, affects every product.
$ProductPaths = @{
    'device-detection' = @('device-detection/', 'examples/device-detection/')
    'ip-intelligence' = @('ip-intelligence/', 'examples/ip-intelligence/')
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
# checkout is not a pull request (a publish, or a run on main).
function Get-AffectedProducts([string]$RepoPath) {
    $head = git -C $RepoPath rev-parse HEAD
    $base = git -C $RepoPath merge-base origin/main HEAD 2>$null
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
$LibraryPaths = @('common', 'device-detection', 'ip-intelligence')

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
