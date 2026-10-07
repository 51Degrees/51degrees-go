# Runs the performance tests.
#
# No performance benchmarks are defined for the generic Pipeline framework;
# performance is measured in the engine repositories (for example device
# detection) that depend on it. This script exists to satisfy the common-ci
# orchestration, which calls it when a build option sets RunPerformance.

param (
    [string]$RepoName
)

Write-Output "No performance tests are defined for $RepoName."
