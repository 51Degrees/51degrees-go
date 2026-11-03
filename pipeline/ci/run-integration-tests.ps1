# Runs the integration tests.
#
# This module has no separate integration test suite: the cloud engine and web
# integration are exercised by the unit tests using in-process httptest servers,
# and the example programs are smoke-tested there too. This script exists to
# satisfy the common-ci orchestration, which expects it to be present.

param (
    [string]$RepoName
)

Write-Output "No separate integration tests for $RepoName; covered by unit tests."
