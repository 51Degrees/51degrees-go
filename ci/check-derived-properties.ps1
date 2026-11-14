<#
.SYNOPSIS
Fails when the committed copy of the derived property scripts differs from the
derived-properties submodule.

.DESCRIPTION
A Go module is built from git's archive of a tag, and an archive leaves a
submodule empty, so the Derived Property Element has to commit its copy of the
scripts it ships. The copy is taken from the derived-properties submodule by
ci/update-packages.ps1. Every script in the submodule's scripts folder must be
in pipeline/elements/derivedproperty/scripts with the same bytes, and the copy
must hold no other file.

.EXAMPLE
git submodule update --init derived-properties
./ci/check-derived-properties.ps1
#>
$ErrorActionPreference = 'Stop'
$PSNativeCommandUseErrorActionPreference = $true

$root = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$source = Join-Path $root 'derived-properties/scripts'
$relative = 'pipeline/elements/derivedproperty/scripts'
$copy = Join-Path $root $relative

$scripts = @(Get-ChildItem -Path $source -File -ErrorAction SilentlyContinue)
if ($scripts.Count -eq 0) {
    throw "The derived-properties submodule has no scripts, so run 'git submodule update --init derived-properties' first"
}
$commit = git -C (Join-Path $root 'derived-properties') rev-parse HEAD

$problems = @()
foreach ($script in $scripts) {
    $copied = Join-Path $copy $script.Name
    if (-not (Test-Path $copied)) {
        $problems += "$relative/$($script.Name) is missing"
        continue
    }
    $expected = (Get-FileHash $script.FullName -Algorithm SHA256).Hash
    $actual = (Get-FileHash $copied -Algorithm SHA256).Hash
    if ($expected -ne $actual) {
        $problems += "$relative/$($script.Name) differs from derived-properties/scripts/$($script.Name)"
    }
}
foreach ($copied in Get-ChildItem -Path $copy -File) {
    if (-not (Test-Path (Join-Path $source $copied.Name))) {
        $problems += "$relative/$($copied.Name) is not in the submodule"
    }
}

if ($problems) {
    foreach ($problem in $problems) {
        Write-Host "::error::$problem"
    }
    throw "The copy of the derived property scripts is not the derived-properties submodule at $commit. Run ci/update-packages.ps1, or copy the files by hand, and commit the result."
}
Write-Host "The copy of the derived property scripts matches derived-properties at $commit"
