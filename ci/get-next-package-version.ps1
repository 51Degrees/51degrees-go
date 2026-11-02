param (
    [Parameter(Mandatory)][string]$RepoName,
    [Parameter(Mandatory)][string]$VariableName
)
$ErrorActionPreference = "Stop"

# The repository has one version, tagged as a plain version such as 4.5.166.
# Each module is also tagged with it, under its folder (see publish-package.ps1).
# Only the patch number moves. The first version is above every version the
# repositories merged into this one released.
$FirstVersion = '4.5.166'

$tags = @(git -C $RepoName tag --list |
    Where-Object { $_ -match '^\d+\.\d+\.\d+$' } |
    Sort-Object { [version]$_ })
$headTags = @(git -C $RepoName tag --points-at HEAD |
    Where-Object { $_ -match '^\d+\.\d+\.\d+$' })

if ($headTags) {
    # Already released, so the publish sees the tag and does nothing.
    $next = $headTags | Sort-Object { [version]$_ } | Select-Object -Last 1
} elseif ($tags) {
    $highest = [version]$tags[-1]
    $next = "$($highest.Major).$($highest.Minor).$($highest.Build + 1)"
} else {
    $next = $FirstVersion
}

Set-Variable -Scope Global -Name $VariableName -Value $next
