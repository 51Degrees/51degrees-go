<#
.SYNOPSIS
Fails when the committed copy of the JavaScript templates differs from the
javascript-templates submodule.

.DESCRIPTION
A Go module is built from git's archive of a tag, and an archive leaves a
submodule empty, so the JavaScript Builder Element has to commit its copy of
the templates. The copy is taken from the javascript-templates submodule by
ci/update-packages.ps1. Every .mustache file at the root of the submodule must
be in pipeline/elements/javascriptbuilder/javascript-templates with the same
bytes, and the copy must hold no other .mustache file.

.EXAMPLE
git submodule update --init javascript-templates
./ci/check-javascript-templates.ps1
#>
$ErrorActionPreference = 'Stop'
$PSNativeCommandUseErrorActionPreference = $true

$root = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$source = Join-Path $root 'javascript-templates'
$relative = 'pipeline/elements/javascriptbuilder/javascript-templates'
$copy = Join-Path $root $relative

$templates = @(Get-ChildItem -Path $source -Filter '*.mustache' -File)
if ($templates.Count -eq 0) {
    throw "The javascript-templates submodule has no templates, so run 'git submodule update --init javascript-templates' first"
}
$commit = git -C $source rev-parse HEAD

$problems = @()
foreach ($template in $templates) {
    $copied = Join-Path $copy $template.Name
    if (-not (Test-Path $copied)) {
        $problems += "$relative/$($template.Name) is missing"
        continue
    }
    $expected = (Get-FileHash $template.FullName -Algorithm SHA256).Hash
    $actual = (Get-FileHash $copied -Algorithm SHA256).Hash
    if ($expected -ne $actual) {
        $problems += "$relative/$($template.Name) differs from javascript-templates/$($template.Name)"
    }
}
foreach ($copied in Get-ChildItem -Path $copy -Filter '*.mustache' -File) {
    if (-not (Test-Path (Join-Path $source $copied.Name))) {
        $problems += "$relative/$($copied.Name) is not in the submodule"
    }
}

if ($problems) {
    foreach ($problem in $problems) {
        Write-Host "::error::$problem"
    }
    throw "The copy of the JavaScript templates is not the javascript-templates submodule at $commit. Run ci/update-packages.ps1, or copy the files by hand, and commit the result."
}
Write-Host "The copy of the JavaScript templates matches javascript-templates at $commit"
