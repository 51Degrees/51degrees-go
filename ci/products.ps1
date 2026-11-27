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

# The example modules of each product. The mixed examples use both engines, so
# they belong to each.
$ProductExamples = @{
    'common' = @()
    'device-detection' = @('examples/device-detection', 'examples/mixed')
    'ip-intelligence' = @('examples/ip-intelligence', 'examples/mixed')
    'pipeline' = @('examples/pipeline')
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

# Returns the figures in go test -bench output, by benchmark name without
# the Benchmark prefix or the processor count go adds, each the average time
# per operation in milliseconds.
function ConvertFrom-GoBenchmark([string[]]$Lines) {
    $figures = [ordered]@{}
    foreach ($line in $Lines) {
        if ($line -match '^Benchmark(\S+?)(?:-\d+)?\s+\d+\s+([\d.]+) ns/op') {
            $figures[$Matches[1]] = [double]$Matches[2] / 1e6
        }
    }
    return $figures
}

# Returns the JavaScript Builder's figures from go test -bench output, under
# LowerIsBetter as the average milliseconds each takes and under HigherIsBetter
# as how many a second. common-ci's performance comparison checks only the
# HigherIsBetter figures. Throws when a benchmark gave no figure.
function Get-JavaScriptBuilderFigures([string[]]$Lines) {
    $figures = ConvertFrom-GoBenchmark $Lines
    $names = [ordered]@{
        'JavaScriptBuilder/Minified' = @(
            'AvgMillisecsPerMinifiedScript', 'MinifiedScriptsPerSecond')
        'JavaScriptBuilder/NotMinified' = @(
            'AvgMillisecsPerScriptNotMinified', 'ScriptsNotMinifiedPerSecond')
        'JavaScriptBuilderMinification' = @(
            'AvgMillisecsPerMinification', 'MinificationsPerSecond')
    }
    $lower = [ordered]@{}
    $higher = [ordered]@{}
    foreach ($benchmark in $names.Keys) {
        if (!$figures.Contains($benchmark)) {
            throw "The JavaScript Builder performance tests gave no figure for $benchmark"
        }
        $lower[$names[$benchmark][0]] = $figures[$benchmark]
        $higher[$names[$benchmark][1]] = 1000 / $figures[$benchmark]
    }
    return @{ LowerIsBetter = $lower; HigherIsBetter = $higher }
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

# How long go test lets one package's tests run, where its default is 10
# minutes. The slowest package, ip-intelligence/ipi_onpremise, takes 140 to
# 180 seconds on Ubuntu, about 200 on macOS and 240 to 300 on Windows, and
# more than 600 on a slow Ubuntu runner, so this is three times that.
$GoTestTimeoutMinutes = 30

# A line in which a compiler, linker or other tool reports a warning, such as
# "file.c:3:5: warning: ..." or "ld: warning: ...". The go command prints such
# lines and still succeeds, so CI fails on them here.
$ToolWarningPattern = '(^|\s)warning:\s'

# Here until the shared CI sets _51DEGREES_WARNINGS_AS_ERRORS in every job.
# Every CI script that builds, vets or tests dot-sources this file, so each of
# them makes C compiler warnings errors. A value already set is kept. This
# folder is in no Go module, so a customer's own go build never reads it.
if (!$env:_51DEGREES_WARNINGS_AS_ERRORS) {
    $env:_51DEGREES_WARNINGS_AS_ERRORS = 'true'
}

# Returns the C compiler flags the go command is run with. When
# _51DEGREES_WARNINGS_AS_ERRORS is true, in any case, that is CGO_CFLAGS with
# -Werror added, so every C compiler warning stops the build. The packages'
# #cgo lines leave -Werror out, as a customer's own build shows a warning and
# goes on. Setting CGO_CFLAGS replaces the go command's default of -g -O2, so
# that default is kept when CGO_CFLAGS is not set or empty. Otherwise
# CGO_CFLAGS is returned as it is.
function Get-CgoCFlags {
    if ($env:_51DEGREES_WARNINGS_AS_ERRORS -ne 'true') { return $env:CGO_CFLAGS }
    $flags = $env:CGO_CFLAGS
    if (!$flags) { $flags = '-g -O2' }
    return "$flags -Werror"
}

# Runs the go command with the given arguments, showing what it prints. Throws
# when it fails or prints a warning. With -PassThru it also returns the lines
# it printed, for a caller that reads them.
function Invoke-Go(
    [Parameter(Mandatory)][string[]]$Arguments,
    [switch]$PassThru
) {
    $lines = [System.Collections.Generic.List[string]]::new()
    $warnings = [System.Collections.Generic.List[string]]::new()
    $savedCFlags = $env:CGO_CFLAGS
    $env:CGO_CFLAGS = Get-CgoCFlags
    try {
        go @Arguments 2>&1 | ForEach-Object {
            $line = $_.ToString()
            Write-Host $line
            $lines.Add($line)
            if ($line -cmatch $ToolWarningPattern) { $warnings.Add($line) }
        }
    } finally {
        $env:CGO_CFLAGS = $savedCFlags
    }
    if ($LASTEXITCODE -ne 0) {
        throw "go $Arguments failed with exit code $LASTEXITCODE"
    }
    if ($warnings.Count) {
        throw "go $Arguments printed warnings:`n$($warnings -join "`n")"
    }
    if ($PassThru) { return $lines.ToArray() }
}

# Runs go test with the given arguments in a module folder, writing JUnit
# results. Returns the exit code, which is not 0 when a test fails or the go
# command or a test prints a warning.
function Invoke-GoTests {
    param(
        [Parameter(Mandatory)][string]$ModulePath,
        [Parameter(Mandatory)][string[]]$Packages,
        [Parameter(Mandatory)][string]$JUnitPath
    )
    # Absolute, because the tests run from the module folder.
    $JUnitPath = [System.IO.Path]::GetFullPath($JUnitPath)
    New-Item -ItemType Directory -Force -Path (Split-Path $JUnitPath) | Out-Null
    $savedCFlags = $env:CGO_CFLAGS
    $env:CGO_CFLAGS = Get-CgoCFlags
    Push-Location $ModulePath
    try {
        Write-Host "Testing $Packages in '$ModulePath'"
        # go test writes what the compiler and linker print, warnings included,
        # to standard error, and go-junit-report counts any of it as a failed
        # build. So standard error goes to the log and only the test output to
        # the report. A test that runs the go command itself, as the examples'
        # tests do, passes on what that command's compiler prints as its own
        # output, and go-junit-report reads a line starting '# ' there as a
        # package that failed to build, so those lines go to the log as well.
        # A package that does not build still fails, because go test reports
        # it on standard output as FAIL. A warning line from either stream
        # fails the run, as the go command passes with it.
        $warnings = [System.Collections.Generic.List[string]]::new()
        go test -v -timeout "$($GoTestTimeoutMinutes)m" @Packages 2>&1 |
        ForEach-Object {
            $line = $_.ToString()
            if ($line -cmatch $ToolWarningPattern) { $warnings.Add($line) }
            if ($_ -is [System.Management.Automation.ErrorRecord] -or
                $line.StartsWith('# ')) {
                Write-Host $line
                return
            }
            $_
        } | go-junit-report -set-exit-code -iocopy -out $JUnitPath | Out-Host
        $code = $LASTEXITCODE
        if ($warnings.Count) {
            Write-Host "Warnings, which fail the tests:`n$($warnings -join "`n")"
            if ($code -eq 0) { $code = 1 }
        }
        return $code
    } finally {
        Pop-Location
        $env:CGO_CFLAGS = $savedCFlags
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
