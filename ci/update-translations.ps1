<#
.SYNOPSIS
Copies the translation data the IP Intelligence translation package embeds from
the cxx submodule.

.DESCRIPTION
ip-intelligence/translation embeds the Translations folder of
ip-intelligence-data. A Go module holds only its own files, so the package
carries a copy of it, made from cxx/ip-intelligence-data, the data the C
sources are tested with.

With -Check, nothing is written, and the script fails if the copy differs from
cxx: a file changed, missing or extra. Line endings are not compared.

.EXAMPLE
./ci/update-translations.ps1
#>
param (
    [switch]$Check
)
$ErrorActionPreference = "Stop"

$root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$source = Join-Path $root "cxx/ip-intelligence-data/Translations"
$target = Join-Path $root "ip-intelligence/translation/Translations"
if (!(Test-Path $source)) { throw "'$source' was not found; check out the cxx submodule with its submodules" }

function Get-Files([string]$Folder) {
    if (!(Test-Path $Folder)) { return @() }
    $files = [string[]]@(Get-ChildItem -Path $Folder -File -Recurse |
        ForEach-Object { [System.IO.Path]::GetRelativePath($Folder, $_.FullName) -replace '\\', '/' })
    [Array]::Sort($files, [StringComparer]::Ordinal)
    return $files
}

$version = git -C (Join-Path $root cxx) describe --tags --always

if ($Check) {
    $wanted = @(Get-Files $source)
    $have = @(Get-Files $target)
    $stale = @($wanted | Where-Object { $have -notcontains $_ }) +
        @($have | Where-Object { $wanted -notcontains $_ })
    foreach ($file in $wanted | Where-Object { $have -contains $_ }) {
        $a = [System.IO.File]::ReadAllText((Join-Path $source $file)) -replace "`r", ''
        $b = [System.IO.File]::ReadAllText((Join-Path $target $file)) -replace "`r", ''
        if ($a -cne $b) { $stale += $file }
    }
    if ($stale) {
        throw "Not copied from cxx $version, run ci/update-translations.ps1: $($stale -join ', ')"
    }
    Write-Host "The translation data matches cxx $version"
    return
}

Remove-Item -Recurse -Force $target -ErrorAction SilentlyContinue
Copy-Item -Recurse $source $target
Write-Host "Copied $(@(Get-Files $target).Count) translation files from cxx $version"
