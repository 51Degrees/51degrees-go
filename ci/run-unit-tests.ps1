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
    'ip-intelligence' = @('./ipi_interop', './ipi_onpremise', './ipi_cloud')
    'pipeline' = @('./...')
}
# The engines' test that data is refreshed while other goroutines process
# reports a reload that is not made on its own only under the race detector,
# so it runs again with the detector on. Linux is enough to find it.
$RaceTest = 'TestRefreshDataWhileProcessing'
$RacePackages = @{
    'device-detection' = './onpremise'
    'ip-intelligence' = './ipi_onpremise'
}
$failed = 0

foreach ($product in Get-AffectedProducts $RepoName $Branch) {
    $env:_51DEGREES_IPI_PATH = $product -eq 'ip-intelligence' ? "$PWD/assets/51Degrees-EnterpriseIpiV41.ipi" : $null
    $code = Invoke-GoTests -ModulePath (Get-ProductPath $RepoName $product) `
        -Packages $Packages[$product] -JUnitPath "$Results/$Name-$product.xml"
    if ($code -ne 0) {
        Write-Host "Unit tests for '$product' failed with exit code $code"
        $failed = $code
    }
    if ($IsLinux -and $RacePackages.ContainsKey($product)) {
        Write-Host "Testing $RaceTest of '$product' with the race detector"
        Push-Location (Get-ProductPath $RepoName $product)
        try {
            go test -race -count=1 -run "^$RaceTest`$" $RacePackages[$product]
            if ($LASTEXITCODE -ne 0) {
                Write-Host "$RaceTest of '$product' failed with the race detector"
                $failed = $LASTEXITCODE
            }
        } finally {
            Pop-Location
        }
    }
}

exit $failed
