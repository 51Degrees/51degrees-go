# Installs the tooling required to run the CI steps for this Go repository.
#
# go-junit-report converts `go test` output into the JUnit XML format that the
# common-ci test-results publishing step expects.

$ErrorActionPreference = "Stop"

Write-Output "Installing go-junit-report..."
go install github.com/jstemmer/go-junit-report/v2@latest || $(throw "go-junit-report installation failed")
go-junit-report -version || $(throw "go-junit-report not available")
