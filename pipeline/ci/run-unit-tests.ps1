# Runs the unit tests and writes the results in JUnit XML format for the
# common-ci test-results publishing step.

param (
    [Parameter(Mandatory)][string]$RepoName
)
$ErrorActionPreference = "Stop"

$commonTestResults = New-Item -ItemType directory -Path $RepoName/test-results/unit -Force

Push-Location $RepoName
try {
    Write-Output "Running unit tests..."
    go test -v 2>&1 ./... | go-junit-report -set-exit-code -iocopy -out $commonTestResults/results.xml || $(throw "unit tests failed")
} finally {
    Pop-Location
}
