# Fetches any assets required by the tests.
#
# The Pipeline API itself has no data-file dependencies - the cloud engine tests
# use an in-process mock server - so there is nothing to fetch. This script
# exists to satisfy the common-ci orchestration, which expects it to be present.

param (
    [string]$RepoName
)

Write-Output "No assets are required for $RepoName."
