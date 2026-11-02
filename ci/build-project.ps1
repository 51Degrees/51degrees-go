param (
    [Parameter(Mandatory)][string]$RepoName
)
$ErrorActionPreference = "Stop"
$PSNativeCommandUseErrorActionPreference = $true

. $PSScriptRoot/products.ps1

& $PSScriptRoot/update-amalgamation.ps1 -Check

# The tests build the packages too. Building first makes a compile error show
# as its own failure.
foreach ($product in Get-AffectedProducts $RepoName) {
    Push-Location (Get-ProductPath $RepoName $product)
    try {
        Write-Host "Building $product"
        go build ./...
    } finally {
        Pop-Location
    }
}
