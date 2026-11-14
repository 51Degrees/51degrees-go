param(
    # Every entry of options.json.
    [Parameter(Mandatory)][object[]]$Options,
    # The files the pull request changes, less documentation.
    [string[]]$ChangedFiles,
    # Those of the changed files whose change touches only comments.
    [string[]]$CommentOnlyFiles
)
# Chooses the options.json entries a pull request is built on, for common-ci's
# steps/select-options.ps1. The branch itself and a release build every entry.
# Within each entry, products.ps1 still builds only the modules the change
# touches.
#
# Every entry but macOS runs, as each takes a few minutes. Only five macOS jobs
# run at once across the organisation, and the releases need them, so macOS is
# left to the branch and release builds, as is performance.

foreach ($entry in $Options) {
    if ($entry.Image -like 'macos*') {
        Write-Host "$($entry.Name) is left to the branch and release builds"
        continue
    }
    $entry.RunPerformance = $false
    Write-Host "$($entry.Name) runs"
    $entry
}
