param (
    [Parameter(Mandatory)][string]$RepoName,
    [Parameter(Mandatory)][string]$Name,
    # The branch a pull request merges into, which its changes are measured
    # against.
    [string]$Branch = 'main'
)
$ErrorActionPreference = "Stop"

. $PSScriptRoot/products.ps1

$Results = Join-Path $RepoName "test-results/unit"
$Packages = @{
    'common' = @('./...')
    'device-detection' = @('./dd', './onpremise')
    'ip-intelligence' = @('./ipi_interop', './ipi_onpremise')
    'pipeline' = @('./...')
}
$failed = 0

foreach ($product in Get-AffectedProducts $RepoName $Branch) {
    $env:DATA_FILE = $product -eq 'ip-intelligence' ? "$PWD/assets/51Degrees-EnterpriseIpiV41.ipi" : $null
    $code = Invoke-GoTests -ModulePath (Get-ProductPath $RepoName $product) `
        -Packages $Packages[$product] -JUnitPath "$Results/$Name-$product.xml"
    if ($code -ne 0) {
        Write-Host "Unit tests for '$product' failed with exit code $code"
        $failed = $code
    }
}

exit $failed
