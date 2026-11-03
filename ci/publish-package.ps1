param (
    [Parameter(Mandatory)][string]$RepoName,
    [Parameter(Mandatory)][string]$Version,
    [bool]$DryRun
)
$ErrorActionPreference = "Stop"
$PSNativeCommandUseErrorActionPreference = $true

# Go finds a version of a module in a folder by a tag named after the folder,
# such as device-detection/v4.5.166, so a release is these tags. The shared
# workflow then tags the repository with the plain version.
$Modules = @('common', 'device-detection', 'ip-intelligence', 'pipeline')

# A published version can't be replaced: sum.golang.org keeps the checksum of
# the first one it sees, so a moved tag breaks every build that uses it.
$tags = $Modules | ForEach-Object { "$_/v$Version" }
foreach ($tag in $tags) {
    if (git -C $RepoName tag --list $tag) { throw "'$tag' already exists" }
    if (git -C $RepoName ls-remote --tags origin "refs/tags/$tag") { throw "'$tag' already exists on origin" }
}

foreach ($tag in $tags) {
    Write-Output "Tagging '$tag'"
    git -C $RepoName tag $tag
}
if ($DryRun) {
    Write-Output "Dry run - not pushing $($tags -join ', ')"
} else {
    Write-Output "Pushing $($tags -join ', ')"
    git -C $RepoName push origin @tags
}
