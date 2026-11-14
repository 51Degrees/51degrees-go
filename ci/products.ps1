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

# The modules each module requires, from packages.json. The workspace builds a
# module against the folders of the modules it requires, so a change to one of
# those has to build and test it too.
$ModuleRequires = @{}
foreach ($module in (Get-Content -Raw (Join-Path $PSScriptRoot 'packages.json') |
        ConvertFrom-Json)) {
    $ModuleRequires[$module.Name] = @($module.DependsOn)
}

# Returns the products the changed paths affect, with every product that
# requires one of them.
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
    do {
        $added = $false
        foreach ($product in $AllProducts) {
            $requires = @($ModuleRequires[$product] |
                Where-Object { $affected.Contains($_) })
            if (!$affected.Contains($product) -and $requires) {
                $affected.Add($product)
                $added = $true
            }
        }
    } while ($added)
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

# The folders of the published modules, from packages.json, which also gives
# each module's version and what it requires (see packages.ps1).
$LibraryPaths = @((Get-Content -Raw (Join-Path $PSScriptRoot 'packages.json') |
    ConvertFrom-Json) | Where-Object Publish | ForEach-Object Path)

# Returns a go line's version in a form that compares correctly, as 1.22 and
# 1.22.0 name the same release.
function ConvertTo-GoVersion([string]$Version) {
    $parts = @($Version.Split('.'))
    while ($parts.Count -lt 3) { $parts += '0' }
    [version]($parts[0..2] -join '.')
}

# Returns the newest version a published module was released at, such as
# 4.5.173, from its Go tags such as common/v4.5.173, or nothing. Tags are read
# by prefix, as PowerShell on Linux and macOS would expand a '*' in an argument
# into file names.
function Get-ModuleRelease([string]$RepoPath, [string]$Library) {
    $versions = foreach ($tag in @(git -C $RepoPath for-each-ref `
            '--format=%(refname:strip=2)' "refs/tags/$Library/")) {
        if ($tag -match "^$([regex]::Escape($Library))/v(\d+\.\d+\.\d+)$") {
            [version]$Matches[1]
        }
    }
    $latest = $versions | Sort-Object | Select-Object -Last 1
    if ($latest) { "$latest" }
}

# Returns the highest go line of the published modules that the module in the
# folder requires, each as it is at its latest release, or nothing.
function Get-ReleaseGoVersion([string]$RepoPath, [string]$ModuleFolder) {
    $mod = go mod edit -json (Join-Path $RepoPath $ModuleFolder 'go.mod') | ConvertFrom-Json
    $versions = foreach ($require in $mod.Require) {
        foreach ($library in $LibraryPaths) {
            if ($require.Path -like "github.com/51Degrees/51degrees-go/$library/v*") {
                $release = Get-ModuleRelease $RepoPath $library
                if (!$release) { continue }
                $line = git -C $RepoPath show "$library/v${release}:$library/go.mod" |
                    Select-String '^go (\S+)'
                $line.Matches[0].Groups[1].Value
            }
        }
    }
    $versions | Sort-Object { ConvertTo-GoVersion $_ } | Select-Object -Last 1
}

# Returns a go mod edit -replace argument for each requirement of the module in
# the folder on a module of this repository at a version with no tag yet. Such
# a version is the one the next release publishes for that module, from its
# folder here, so the folder is what that version will hold.
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
