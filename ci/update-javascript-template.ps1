<#
.SYNOPSIS
Copies the shared JavaScript template from the javascript-templates submodule
into the JavaScript Builder Element's package.

.DESCRIPTION
The JavaScript Builder Element renders javascript-templates'
JavaScriptResource.mustache, as every 51Degrees language does. A Go module
holds only the files committed in its own folder, never a submodule's, so the
package embeds a committed copy of the file.

With -Check, nothing is written, and the script fails if the committed copy is
not the file at the submodule's commit. Git's object ids are compared, so line
endings a checkout converts make no difference.

.EXAMPLE
./ci/update-javascript-template.ps1
#>
param (
    [switch]$Check
)
$ErrorActionPreference = "Stop"
$PSNativeCommandUseErrorActionPreference = $true

$root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$templates = Join-Path $root javascript-templates
$name = 'JavaScriptResource.mustache'
$copy = 'pipeline/elements/javascriptbuilder/JavaScriptResource.mustache'

# An empty folder would make git look in this repository instead.
if (!(Test-Path (Join-Path $templates .git))) {
    throw "javascript-templates is not checked out, run git submodule update --init javascript-templates"
}
$commit = git -C $templates rev-parse HEAD
$source = git -C $templates rev-parse "HEAD:$name"

if ($Check) {
    $committed = git -C $root hash-object $copy
    if ($committed -ne $source) {
        throw "$copy is not $name from javascript-templates $commit, run ci/update-javascript-template.ps1"
    }
    Write-Host "$copy matches javascript-templates $commit"
} else {
    Copy-Item (Join-Path $templates $name) (Join-Path $root $copy)
    Write-Host "Copied $name from javascript-templates $commit to $copy"
}
