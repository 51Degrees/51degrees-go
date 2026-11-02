<#
.SYNOPSIS
Generates the C sources each Go engine compiles from the cxx submodule.

.DESCRIPTION
cgo compiles the C files in a package's own folder, and a Go module holds only
its own folder, so each engine carries the 51degrees-cxx sources it needs as one
amalgamated .c file and one .h file, made by ci/amalgamate.awk. Each file starts
with the cxx version it was made from.

With -Check, nothing is written, and the script fails if a committed file
differs from what it would generate. Line endings are not compared.

Needs awk, which Git for Windows provides.

.EXAMPLE
./ci/update-amalgamation.ps1
#>
param (
    [switch]$Check
)
$ErrorActionPreference = "Stop"
$PSNativeCommandUseErrorActionPreference = $true

$root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$src = "cxx/src"

$awk = (Get-Command awk -ErrorAction SilentlyContinue).Source
if (!$awk -and $IsWindows) { $awk = "$env:ProgramFiles/Git/usr/bin/awk.exe" }
if (!$awk -or !(Test-Path $awk)) { throw "awk was not found" }

# The same order on every platform.
function Get-Sources([string]$Pattern) {
    $files = [string[]]@(Get-ChildItem -Path (Join-Path $root $Pattern) -File |
        ForEach-Object { [System.IO.Path]::GetRelativePath($root, $_.FullName) -replace '\\', '/' })
    [Array]::Sort($files, [StringComparer]::Ordinal)
    return $files
}

$outputs = [ordered]@{
    'device-detection/dd/device-detection-cxx.c' =
        (Get-Sources "$src/common/*.c") + (Get-Sources "$src/dd/*.c") + (Get-Sources "$src/dd/hash/*.c")
    'device-detection/dd/device-detection-cxx.h' =
        @("$src/dd/hash/fiftyone.h", "$src/dd/hash/hash.h", "$src/common/fiftyone.h")
    'ip-intelligence/ipi_interop/ip-intelligence-cxx.c' =
        (Get-Sources "$src/common/*.c") + (Get-Sources "$src/ipi/graph/*.c") + (Get-Sources "$src/ipi/*.c")
    'ip-intelligence/ipi_interop/ip-intelligence-cxx.h' =
        @("$src/ipi/fiftyone.h", "$src/ipi/graph/graph.h")
}

$version = git -C (Join-Path $root cxx) describe --tags --always
$commit = git -C (Join-Path $root cxx) rev-parse HEAD
$header = "/* Generated from 51degrees-cxx $version ($commit) by ci/update-amalgamation.ps1. Do not edit by hand. */"

$stale = @()
Push-Location $root
try {
    foreach ($output in $outputs.Keys) {
        $generated = [System.IO.Path]::GetTempFileName()
        try {
            & $awk -f ci/amalgamate.awk @($outputs[$output]) > $generated
            $text = $header + "`n" + [System.IO.File]::ReadAllText($generated)
        } finally {
            Remove-Item $generated
        }
        if ($Check) {
            $committed = [System.IO.File]::ReadAllText((Join-Path $root $output))
            if (($committed -replace "`r", '') -cne ($text -replace "`r", '')) {
                $stale += $output
            }
        } else {
            [System.IO.File]::WriteAllText((Join-Path $root $output), $text)
            Write-Host "Generated $output"
        }
    }
} finally {
    Pop-Location
}

if ($stale) {
    throw "Not generated from cxx $version, run ci/update-amalgamation.ps1: $($stale -join ', ')"
}
if ($Check) {
    Write-Host "The amalgamated sources match cxx $version"
}
