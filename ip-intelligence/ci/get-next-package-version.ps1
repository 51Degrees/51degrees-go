param (
    [Parameter(Mandatory)][string]$RepoName,
    [Parameter(Mandatory)][string]$VariableName
)
./go/get-next-package-version.ps1 @PSBoundParameters
