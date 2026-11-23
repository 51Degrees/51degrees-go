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

A script listed in ci/derived-properties-reworded.json has some comment lines
reworded in the copy, for the reason the list gives. Its copy is checked
against the submodule's script at the blob the list records. Every line that
is not a comment must be the submodule's, in the same place, and the copy must
differ, or the script would not be listed. When the submodule's script
changes, its blob no longer matches and the check fails, so the new version is
copied and reworded by hand and its blob recorded.

.EXAMPLE
git submodule update --init derived-properties
./ci/check-derived-properties.ps1
#>
$ErrorActionPreference = 'Stop'
$PSNativeCommandUseErrorActionPreference = $true

$root = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$submodule = Join-Path $root 'derived-properties'
$source = Join-Path $submodule 'scripts'
$relative = 'pipeline/elements/derivedproperty/scripts'
$copy = Join-Path $root $relative
$reworded = Get-Content (Join-Path $PSScriptRoot 'derived-properties-reworded.json') -Raw |
    ConvertFrom-Json

$scripts = @(Get-ChildItem -Path $source -File -ErrorAction SilentlyContinue)
if ($scripts.Count -eq 0) {
    throw "The derived-properties submodule has no scripts, so run 'git submodule update --init derived-properties' first"
}
$commit = git -C $submodule rev-parse HEAD

# Whether a line of a script is a YAML comment.
function Test-Comment([string]$Line) { $Line -match '^\s*#' }

$problems = @()
foreach ($name in $reworded.PSObject.Properties.Name) {
    if (-not (Test-Path (Join-Path $source $name))) {
        $problems += "$name is listed in ci/derived-properties-reworded.json and is not in the submodule"
    }
}
foreach ($script in $scripts) {
    $copied = Join-Path $copy $script.Name
    if (-not (Test-Path $copied)) {
        $problems += "$relative/$($script.Name) is missing"
        continue
    }
    $expected = (Get-FileHash $script.FullName -Algorithm SHA256).Hash
    $actual = (Get-FileHash $copied -Algorithm SHA256).Hash
    $listed = $reworded.PSObject.Properties[$script.Name]
    if (-not $listed) {
        if ($expected -ne $actual) {
            $problems += "$relative/$($script.Name) differs from derived-properties/scripts/$($script.Name)"
        }
        continue
    }
    $blob = git -C $submodule rev-parse "HEAD:scripts/$($script.Name)"
    if ($blob -ne $listed.Value.Blob) {
        $problems += "derived-properties/scripts/$($script.Name) is blob $blob, not $($listed.Value.Blob), so copy it again, reword its comments and record its blob in ci/derived-properties-reworded.json"
        continue
    }
    if ($expected -eq $actual) {
        $problems += "$relative/$($script.Name) is listed as reworded and is the submodule's script"
        continue
    }
    $originalLines = @(Get-Content $script.FullName)
    $copiedLines = @(Get-Content $copied)
    if ($originalLines.Count -ne $copiedLines.Count) {
        $problems += "$relative/$($script.Name) has $($copiedLines.Count) lines and the submodule's has $($originalLines.Count)"
        continue
    }
    for ($i = 0; $i -lt $originalLines.Count; $i++) {
        if ($originalLines[$i] -cne $copiedLines[$i] -and
            -not ((Test-Comment $originalLines[$i]) -and (Test-Comment $copiedLines[$i]))) {
            $problems += "$relative/$($script.Name) line $($i + 1) differs from the submodule's and is not a comment"
        }
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
