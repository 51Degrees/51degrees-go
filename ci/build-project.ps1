param (
    [Parameter(Mandatory)][string]$RepoName
)
$ErrorActionPreference = "Stop"
$PSNativeCommandUseErrorActionPreference = $true

. $PSScriptRoot/products.ps1

& $PSScriptRoot/update-amalgamation.ps1 -Check

# The tests build the packages too. Building first makes a compile error show
# as its own failure. Each module is also built on its own, without the
# workspace, as the published module is. A requirement on a module of this
# repository at the version the next release publishes is not published yet,
# so it is taken from that module's folder, through a copy of go.mod.
$repoRoot = (Resolve-Path $RepoName).Path
foreach ($product in Get-AffectedProducts $RepoName) {
    $replaces = @(Get-UnreleasedRequirementReplaces $repoRoot $product)
    Push-Location (Get-ProductPath $RepoName $product)
    try {
        Write-Host "Building $product"
        go build ./...
        Write-Host "Building $product without the workspace"
        $env:GOWORK = 'off'
        if ($replaces) {
            Write-Host "Taking requirements the next release publishes from this repository: $($replaces -join ' ')"
            Copy-Item go.mod go.unreleased.mod
            Copy-Item go.sum go.unreleased.sum -ErrorAction SilentlyContinue
            go mod edit @replaces go.unreleased.mod
            # Quoted, because PowerShell splits an unquoted -name=value argument
            # at its first dot.
            go build "-modfile=go.unreleased.mod" ./...
        } else {
            go build ./...
        }
    } finally {
        Remove-Item Env:GOWORK -ErrorAction SilentlyContinue
        Remove-Item go.unreleased.mod, go.unreleased.sum -ErrorAction SilentlyContinue
        Pop-Location
    }
}
