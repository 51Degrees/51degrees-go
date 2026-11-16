$ErrorActionPreference = "Stop"

./go/setup-environment.ps1

. $PSScriptRoot/products.ps1
. $PSScriptRoot/packages.ps1

# Moves each requirement of an example on a published module of this
# repository that names an earlier release than that module's latest.
function Move-ExampleToRelease([string]$Folder, [hashtable]$Releases) {
    $mod = go mod edit -json (Join-Path $Folder go.mod) | ConvertFrom-Json
    $moves = @(foreach ($require in $mod.Require) {
        $library = $LibraryPaths | Where-Object {
            $require.Path -like "github.com/51Degrees/51degrees-go/$_/v*"
        }
        if ($library -and $Releases[$library] -and
            $require.Version -match '^v(\d+\.\d+\.\d+)$' -and
            [version]$Matches[1] -lt [version]$Releases[$library]) {
            "$($require.Path)@v$($Releases[$library])"
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

$repo = Join-Path $PSScriptRoot ".."

# A fresh clone builds the examples with the committed go.work, so it must be
# what create-workspace.ps1 makes from the go.mod files. git status, not git
# diff, so that a go.work missing from the commit, or ignored, fails as well.
& $PSScriptRoot/create-workspace.ps1
$status = git -C $repo status --porcelain --ignored -- go.work
if ($status) {
    Write-Host $status
    git -C $repo --no-pager diff -- go.work | Out-Host
    throw "The committed go.work isn't what ci/create-workspace.ps1 makes, so run it and commit go.work"
}

# When the release plan has a module in it, the tests are of the next release,
# so the examples use the workspace. Otherwise they are of the latest releases,
# so the examples use the published modules, each at its latest release, as a
# customer's code does when it takes them. An example's go.mod can name an
# earlier release, because the nightly package update moves the examples on rc
# and main has the move only after its next release, so such a requirement is
# moved first. Nothing is committed.
$plan = @(Get-ModuleReleasePlan $repo)
if ($plan.Count -gt 0) {
    Write-Host "$($plan.Name -join ', ') would be released, so the examples use the modules in this repository"
} else {
    Remove-Item (Join-Path $repo go.work), (Join-Path $repo go.work.sum) -ErrorAction SilentlyContinue
    $releases = @{}
    foreach ($library in $LibraryPaths) {
        $releases[$library] = Get-ModuleRelease $repo $library
    }
    $named = ($LibraryPaths | ForEach-Object { "$_ $($releases[$_])" }) -join ', '
    Write-Host "No published module has changed since its last release, so the examples use the published modules at $named"
    $env:GOWORK = 'off'
    try {
        foreach ($example in 'device-detection', 'ip-intelligence', 'mixed', 'pipeline') {
            Move-ExampleToRelease (Join-Path $repo examples $example) $releases
        }
    } finally {
        Remove-Item Env:GOWORK
    }
}
