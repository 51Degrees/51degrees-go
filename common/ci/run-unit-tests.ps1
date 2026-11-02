param (
    [Parameter(Mandatory)][string]$RepoName
)
./go/run-unit-tests.ps1 -RepoName $RepoName
