param (
    [Parameter(Mandatory)][string]$RepoName,
    [Parameter(Mandatory)][string]$OrgName,
    [bool]$DryRun
)
$ErrorActionPreference = "Stop"
$PSNativeCommandUseErrorActionPreference = $true
. (Join-Path $PSScriptRoot 'packages.ps1')

# The repository on GitHub, which Go downloads this repository's modules from.
$RepositoryUrl = "https://github.com/$OrgName/$RepoName"

# Makes a commit on $Commit that raises a module's requirements to versions
# tagged earlier in this publish, with go.sum to match, and returns it. Go
# fetches this repository's modules from the checkout here rather than from
# GitHub, so it finds those tags before they are pushed, and go.sum gets the
# checksums sum.golang.org will hold for them.
function New-RequirementCommit {
    param (
        [string]$RepoPath,
        $Package,
        [string]$Commit,
        [string[]]$Requirements,
        [string]$Version
    )
    $repo = (Resolve-Path $RepoPath).Path
    $worktree = Join-Path ([IO.Path]::GetTempPath()) `
        "go-release-$([guid]::NewGuid().ToString('N'))"
    git -C $repo worktree add --quiet --detach $worktree $Commit

    $environment = @{
        GOWORK = 'off'
        GOPRIVATE = "github.com/$OrgName/$RepoName"
        GIT_CONFIG_COUNT = '1'
        GIT_CONFIG_KEY_0 = "url.$repo.insteadOf"
        GIT_CONFIG_VALUE_0 = $RepositoryUrl
    }
    $saved = @{}
    foreach ($name in $environment.Keys) {
        $saved[$name] = [Environment]::GetEnvironmentVariable($name)
        [Environment]::SetEnvironmentVariable($name, $environment[$name])
    }
    Push-Location (Join-Path $worktree $Package.Path)
    try {
        # Out-Host, so that only the commit is returned.
        foreach ($requirement in $Requirements) {
            go mod edit "-require=$requirement" | Out-Host
        }
        go mod tidy | Out-Host
        git add go.mod go.sum | Out-Host
        git commit --quiet -m "Require $($Requirements -join ', ') in $($Package.Name) $Version" |
            Out-Host
        return (git rev-parse HEAD)
    } finally {
        Pop-Location
        # Restored before anything is pushed, as the URL rewrite would send a
        # push to the checkout.
        foreach ($name in $saved.Keys) {
            [Environment]::SetEnvironmentVariable($name, $saved[$name])
        }
        git -C $repo worktree remove --force $worktree
    }
}

# Each module is released by its own tag and GitHub release, in place of
# common-ci's single tag, which would name no module.
$global:SkipUpdateTag = $true

# Read before anything is tagged, because the tags made below change it.
$plan = @(Get-ModuleReleasePlan $RepoName)
if ($plan.Count -eq 0) {
    Write-Output "No module has changed since its last release, so nothing is tagged"
    return
}
$packages = Get-Packages $RepoName

# A published version can't be replaced. sum.golang.org keeps the checksum of
# the first one it sees, so a moved tag breaks every build that uses it.
foreach ($release in $plan) {
    $tag = Get-GoTag ($packages | Where-Object Name -eq $release.Name) $release.Version
    if (git -C $RepoName tag --list $tag) {
        throw "'$tag' already exists"
    }
    if (git -C $RepoName ls-remote --tags origin "refs/tags/$tag") {
        throw "'$tag' already exists on origin"
    }
}

# Go reads a module's requirements from its go.mod at the tag. So a module
# that requires one released in this publish is tagged on a commit that
# raises the requirement to the new version. That commit is reached only from
# the tag, and the nightly package update makes the same change on rc. The
# other modules are tagged on HEAD. The plan lists each module after those it
# requires, so their tags exist when it is reached.
$base = git -C $RepoName rev-parse HEAD
$tip = $base
$versions = @{}
$releases = [Collections.Generic.List[object]]::new()
foreach ($release in $plan) {
    $package = $packages | Where-Object Name -eq $release.Name
    $requirements = @(foreach ($name in $package.DependsOn) {
        if ($versions.ContainsKey($name) -eq $true) {
            $dependency = $packages | Where-Object Name -eq $name
            "$((Read-GoMod $RepoName $dependency).Module)@v$($versions[$name])"
        }
    })
    $commit = $base
    if ($requirements.Count -gt 0) {
        $tip = New-RequirementCommit -RepoPath $RepoName -Package $package `
            -Commit $tip -Requirements $requirements -Version $release.Version
        $commit = $tip
    }
    $tag = Get-GoTag $package $release.Version
    git -C $RepoName tag $tag $commit
    $versions[$release.Name] = $release.Version
    $releases.Add([pscustomobject]@{
        Release = $release
        Package = $package
        Tag = $tag
        Commit = $commit
        Requirements = $requirements
    })
    if ($requirements.Count -gt 0) {
        Write-Output "Tagged '$tag' on $commit, which requires $($requirements -join ', ')"
        git -C $RepoName --no-pager show --format='%H %s' $commit -- `
            "$($package.Path)/go.mod" "$($package.Path)/go.sum"
    } else {
        Write-Output "Tagged '$tag' on $commit"
    }
}

$tags = @($releases.Tag)
if ($DryRun -eq $true) {
    Write-Output "Dry run - tagged $($tags -join ', ') locally, not pushed, no release made"
    return
}
Write-Output "Pushing $($tags -join ', ')"
git -C $RepoName push --atomic origin @tags

# One GitHub release per tag. The notes list the commits that touched the
# module's folder since its last release.
foreach ($entry in $releases) {
    $release = $entry.Release
    $notes = "$($release.Name) $($release.Version), $($release.Reason)."
    if ($entry.Requirements.Count -gt 0) {
        $notes += " Requires $($entry.Requirements -join ', ')."
    }
    if ($null -ne $release.Previous) {
        $range = "$(Get-GoTag $entry.Package $release.Previous)..$base"
        $commits = git -C $RepoName log --format='- %s (%h)' $range `
            -- (Get-PackagePaths $entry.Package)
        if ($commits) {
            $notes += "`n`n" + ($commits -join "`n")
        }
    }
    gh -R "$OrgName/$RepoName" release create $entry.Tag --verify-tag `
        --title "$($release.Name) $($release.Version)" --notes $notes
}
