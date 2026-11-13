$ErrorActionPreference = "Stop"

./go/setup-environment.ps1

. $PSScriptRoot/products.ps1

# Moves each requirement of an example on a published module of this
# repository that names an earlier release to the given release.
function Move-ExampleToRelease([string]$Folder, [string]$Release) {
    $mod = go mod edit -json (Join-Path $Folder go.mod) | ConvertFrom-Json
    $moves = @(foreach ($require in $mod.Require) {
        $published = $LibraryPaths | Where-Object {
            $require.Path -like "github.com/51Degrees/51degrees-go/$_/v*"
        }
        if ($published -and $require.Version -match '^v(\d+\.\d+\.\d+)$' -and
            [version]$Matches[1] -lt [version]$Release) {
            "$($require.Path)@v$Release"
        }
    })
    if (!$moves) { return }
    Write-Host "Moving $Folder to $($moves -join ' ')"
    Push-Location $Folder
    try {
        go get @moves
        if ($LASTEXITCODE -ne 0) { throw "go get failed in $Folder" }
        go mod tidy
        if ($LASTEXITCODE -ne 0) { throw "go mod tidy failed in $Folder" }
    } finally {
        Pop-Location
    }
}

# When a published module has changed since the latest release, the tests are
# of the next release, so the examples use the modules in this repository.
# Otherwise they are of the latest release, so the examples use the published
# modules at that version, as a customer's code does when it takes the release.
# An example's go.mod can name an earlier release, because the nightly package
# update moves the examples on rc and main has the move only after its next
# release, so such a requirement is moved first. Nothing is committed.
$repo = Join-Path $PSScriptRoot ".."
Remove-Item (Join-Path $repo go.work), (Join-Path $repo go.work.sum) -ErrorAction SilentlyContinue
if (Test-UnreleasedLibraryChange $repo) {
    & $PSScriptRoot/create-workspace.ps1
} else {
    $release = Get-LatestRelease $repo
    Write-Host "No published module has changed since $release, so the examples use the published modules at $release"
    $env:GOWORK = 'off'
    try {
        foreach ($example in 'device-detection', 'ip-intelligence', 'mixed', 'pipeline') {
            Move-ExampleToRelease (Join-Path $repo examples $example) $release
        }
    } finally {
        Remove-Item Env:GOWORK
    }
}
