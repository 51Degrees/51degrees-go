<#
.SYNOPSIS
Creates a go.work at the repository root that uses every module in it.

.DESCRIPTION
Each module requires the others at a published version, so an example copied
out of the repository works as a customer's code would. The workspace makes
them use the modules in this repository instead. go.work is not committed.

The go line is the highest of the modules', so the workspace doesn't ask for a
newer Go than they do.

A module that requires another of these modules at a version not published
yet would still make Go look for that version, so each such requirement is
also replaced by the module's folder.
#>
$ErrorActionPreference = "Stop"
$PSNativeCommandUseErrorActionPreference = $true

$root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$modules = @(
    'common'
    'device-detection'
    'ip-intelligence'
    'examples/device-detection'
    'examples/ip-intelligence'
    'examples/mixed'
)

$version = $modules | ForEach-Object {
    (Select-String -Path (Join-Path $root "$_/go.mod") -Pattern '^go (\S+)').Matches[0].Groups[1].Value
} | Sort-Object { [version]$_ } | Select-Object -Last 1

Push-Location $root
try {
    Remove-Item go.work, go.work.sum -ErrorAction SilentlyContinue
    go work init @($modules | ForEach-Object { "./$_" })
    go work edit "-go=$version" -toolchain=none

    $folders = @{}
    $requires = foreach ($module in $modules) {
        $mod = go mod edit -json "$module/go.mod" | ConvertFrom-Json
        $folders[$mod.Module.Path] = "./$module"
        if ($mod.Require) { $mod.Require }
    }
    foreach ($require in $requires | Where-Object { $folders.ContainsKey($_.Path) } |
        Sort-Object Path, Version -Unique) {
        go work edit "-replace=$($require.Path)@$($require.Version)=$($folders[$require.Path])"
    }
    Write-Host "Created go.work (go $version) using: $($modules -join ', ')"
} finally {
    Pop-Location
}
