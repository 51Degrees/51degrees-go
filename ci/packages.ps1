# Dot-sourced by the scripts in this folder. Works out which Go modules a
# publish releases, and at which version, with common-ci's release plan, which
# reads the modules from packages.json. Each module has its own version, and a
# module is released when a file that counts in its folder changed since its
# last tag, or when a module it requires is released. Tests, their data,
# Markdown files, documentation and licences do not count (the Ignore lists in
# packages.json and the release plan's defaults).
#
# common-ci runs these scripts from its own root, with this repository checked
# out inside it, which is where the release plan is looked for first. A
# checkout of common-ci beside this repository is found too, or one elsewhere
# can be named with COMMON_CI.
$releasePlan = @(
    $env:COMMON_CI,
    (Join-Path $PSScriptRoot '..' '..'),
    (Join-Path $PSScriptRoot '..' '..' 'common-ci')) |
    Where-Object { $_ } |
    ForEach-Object { Join-Path $_ 'steps' 'release-plan.ps1' } |
    Where-Object { Test-Path $_ } |
    Select-Object -First 1
if ($null -eq $releasePlan) {
    throw "common-ci's steps/release-plan.ps1 was not found. Set COMMON_CI " +
        "to a checkout of common-ci."
}
. $releasePlan
Remove-Variable releasePlan

# The tag Go reads a version of a module from, the module's folder then 'v'
# and the version, such as device-detection/v4.5.172.
function Get-GoTag($Package, [string]$Version) {
    return "$($Package.Path)/v$Version"
}

# The module path and the requirements in a module's go.mod. Read as text, so
# no Go is needed.
function Read-GoMod([string]$RepoPath, $Package) {
    $text = Get-Content -Raw (Join-Path $RepoPath $Package.Path 'go.mod')
    $requires = @{}
    foreach ($match in [regex]::Matches($text,
            '(?m)^\s*(?:require\s+)?(\S+)\s+(v\d\S*)')) {
        $requires[$match.Groups[1].Value] = $match.Groups[2].Value
    }
    return [pscustomobject]@{
        Module = [regex]::Match($text, '(?m)^module\s+(\S+)').Groups[1].Value
        Requires = $requires
    }
}

# Stops when a module's go.mod and packages.json disagree about which of these
# modules it requires, because the plan would then leave out a module that
# must be released with another, or when it requires one at a version with no
# tag, which nobody could download.
function Assert-ModuleRequirements([string]$RepoPath, $Packages) {
    $published = @($Packages | Where-Object { $_.Publish -eq $true })
    $mods = @{}
    $byModule = @{}
    foreach ($package in $published) {
        $mods[$package.Name] = Read-GoMod $RepoPath $package
        $byModule[$mods[$package.Name].Module] = $package
    }
    foreach ($package in $published) {
        $requires = $mods[$package.Name].Requires
        $required = @(foreach ($path in $requires.Keys) {
            if ($byModule.ContainsKey($path) -eq $false) {
                continue
            }
            $dependency = $byModule[$path]
            $tag = Get-GoTag $dependency $requires[$path].TrimStart('v')
            if (-not (git -C $RepoPath tag --list $tag)) {
                throw "$($package.Path)/go.mod requires $path " +
                    "$($requires[$path]), which has no tag $tag."
            }
            $dependency.Name
        })
        $inGoMod = (@($required | Sort-Object) -join ', ')
        $inPackages = (@($package.DependsOn | Sort-Object) -join ', ')
        if ($inGoMod -ne $inPackages) {
            throw "$($package.Path)/go.mod requires [$inGoMod] of the modules " +
                "here, but ci/packages.json says $($package.Name) depends on " +
                "[$inPackages]."
        }
    }
}

# common-ci's release plan reads tags named '<package>/<version>', and Go's
# tags have a 'v' before the version. So each module's newest Go tag is given
# a local tag of the plan's form, which is never pushed.
#
# A module that requires one released with it is tagged on a commit that
# raises its go.mod (see publish-package.ps1), which the branch only gets later
# from the nightly package update. So the local tag goes on the tagged commit
# when no file that counts differs there from HEAD, and otherwise on the
# commit the release was made from, so that the raise alone is not a change.
function Sync-PlanTags([string]$RepoPath, $Packages) {
    Remove-PlanTags $RepoPath $Packages
    # A failing merge-base is an answer, checked below. Set for this function
    # only.
    $PSNativeCommandUseErrorActionPreference = $false
    foreach ($package in @($Packages | Where-Object { $_.Publish -eq $true })) {
        $prefix = "$($package.Path)/v"
        $goTags = @(git -C $RepoPath for-each-ref '--format=%(refname:strip=2)' `
            "refs/tags/$($package.Path)/")
        $versions = @(@(foreach ($tag in $goTags) {
            if ($tag.StartsWith($prefix)) {
                ConvertTo-VersionParts $tag.Substring($prefix.Length)
            }
        }) | Where-Object { $null -ne $_ } | Sort-Object)
        if ($versions.Count -eq 0) {
            continue
        }

        $last = $versions[-1]
        $tagged = git -C $RepoPath rev-parse "$(Get-GoTag $package $last)^{commit}"
        $at = $tagged
        $changed = @(Get-ChangedFiles $RepoPath $package $tagged)
        $counted = @(Select-CountedFiles $RepoPath $package $tagged $changed)
        if ($counted.Count -gt 0) {
            $base = git -C $RepoPath merge-base $tagged HEAD
            if ($LASTEXITCODE -eq 0 -and $base) {
                $at = $base
            }
        }
        git -C $RepoPath tag "$($package.Name)/$last" $at
        $global:LASTEXITCODE = 0
    }
}

# Deletes the local tags Sync-PlanTags makes, so that a step reading every tag
# of a module sees only its Go tags.
function Remove-PlanTags([string]$RepoPath, $Packages) {
    $PSNativeCommandUseErrorActionPreference = $false
    foreach ($package in @($Packages | Where-Object { $_.Publish -eq $true })) {
        $tags = @(git -C $RepoPath for-each-ref '--format=%(refname:strip=2)' `
            "refs/tags/$($package.Name)/")
        foreach ($tag in $tags) {
            $version = $tag.Substring($package.Name.Length + 1)
            if ($null -ne (ConvertTo-VersionParts $version)) {
                git -C $RepoPath tag -d $tag | Out-Null
            }
        }
    }
}

# What a publish of HEAD releases, from common-ci's Get-ReleasePlan: one entry
# per module, in the order to tag them, each with its version and why.
function Get-ModuleReleasePlan([string]$RepoPath) {
    $packages = Get-Packages $RepoPath
    Assert-ModuleRequirements $RepoPath $packages
    Sync-PlanTags $RepoPath $packages
    return @(Get-ReleasePlan $RepoPath $packages)
}
