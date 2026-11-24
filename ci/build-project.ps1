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
& $PSScriptRoot/update-translations.ps1 -Check

# The tests build the packages too. Building first makes a compile error show
# as its own failure. Each module is also built on its own, without the
# workspace, as the published module is. A requirement on a module of this
# repository at the version the next release publishes is not published yet,
# so it is taken from that module's folder, through a copy of go.mod.
$repoRoot = (Resolve-Path $RepoName).Path
$products = @(Get-AffectedProducts $RepoName $Branch)
foreach ($product in $products) {
    $replaces = @(Get-UnreleasedRequirementReplaces $repoRoot $product)
    Push-Location (Get-ProductPath $RepoName $product)
    try {
        Write-Host "Building $product"
        Invoke-Go @('build', './...')
        Write-Host "Building $product without the workspace"
        $env:GOWORK = 'off'
        if ($replaces) {
            Write-Host "Taking requirements the next release publishes from this repository: $($replaces -join ' ')"
            Copy-Item go.mod go.unreleased.mod
            Copy-Item go.sum go.unreleased.sum -ErrorAction SilentlyContinue
            go mod edit @replaces go.unreleased.mod
            Invoke-Go @('build', '-modfile=go.unreleased.mod', './...')
        } else {
            Invoke-Go @('build', './...')
        }
    } finally {
        Remove-Item Env:GOWORK -ErrorAction SilentlyContinue
        Remove-Item go.unreleased.mod, go.unreleased.sum -ErrorAction SilentlyContinue
        Pop-Location
    }
}

# go vet checks each module and its examples, tests included, and any finding
# or warning fails the build. go test runs only some of its checks. Each module
# is checked before the build fails, so one run shows every finding.
$vetFailed = @()
$modules = @($products) + @($products | ForEach-Object { $ProductExamples[$_] }) |
    Select-Object -Unique
foreach ($module in $modules) {
    Push-Location (Join-Path $RepoName $module)
    try {
        Write-Host "Vetting $module"
        Invoke-Go @('vet', './...')
    } catch {
        Write-Host $_
        $vetFailed += $module
    } finally {
        Pop-Location
    }
}
if ($vetFailed) {
    throw "go vet reported findings in $($vetFailed -join ', ')"
}
