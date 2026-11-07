param (
    [Parameter(Mandatory)][string]$RepoName,
    # The branch a pull request merges into, which its changes are measured
    # against.
    [string]$Branch = 'main'
)
$ErrorActionPreference = "Stop"
$PSNativeCommandUseErrorActionPreference = $true

. $PSScriptRoot/products.ps1

& $PSScriptRoot/update-amalgamation.ps1 -Check

# The tests build the packages too. Building first makes a compile error show
# as its own failure. Each module is also built on its own, without the
# workspace, as the published module is.
foreach ($product in Get-AffectedProducts $RepoName $Branch) {
    Push-Location (Get-ProductPath $RepoName $product)
    try {
        Write-Host "Building $product"
        go build ./...
        Write-Host "Building $product without the workspace"
        $env:GOWORK = 'off'
        go build ./...
    } finally {
        Remove-Item Env:GOWORK -ErrorAction SilentlyContinue
        Pop-Location
    }
}
