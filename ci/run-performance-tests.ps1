param (
    [Parameter(Mandatory)][string]$RepoName,
    [Parameter(Mandatory)][string]$Name
)
$ErrorActionPreference = "Stop"
$PSNativeCommandUseErrorActionPreference = $true

. $PSScriptRoot/products.ps1

$rootDir = $PWD
$Products = Get-AffectedProducts $RepoName

# Every engine's figures go into one results file for this options entry,
# which is what the performance comparison reads. The metric names differ
# between engines, as in 51degrees-cxx.
$higher = [ordered]@{}
$lower = [ordered]@{}

if ($Products -contains 'device-detection') {
    # Absolute, because the example runs from its module folder.
    $output = Join-Path $rootDir "summary-device-detection.json"
    Remove-Item $output -ErrorAction SilentlyContinue
    Push-Location (Join-Path $RepoName 'examples/device-detection')
    try {
        Write-Host "Running the Device Detection performance example..."
        go run ./dd/performance -json-output $output `
            -data-file "$rootDir/assets/51Degrees-LiteV4.1.hash" `
            -evidence-file "$rootDir/assets/20000 Evidence Records.yml"
    } finally {
        Pop-Location
    }
    $results = Get-Content $output | ConvertFrom-Json -AsHashtable
    $higher.DetectionsPerSecond = $results.HigherIsBetter.DetectionsPerSecond
    $lower.AvgMillisecsPerDetection = $results.LowerIsBetter.AvgMillisecsPerDetection
}

if ($Products -contains 'ip-intelligence') {
    $output = Join-Path $rootDir "summary-ip-intelligence.json"
    Remove-Item $output -ErrorAction SilentlyContinue
    $env:_51DEGREES_IPI_PATH = "$rootDir/assets/51Degrees-EnterpriseIpiV41.ipi"
    Push-Location (Join-Path $RepoName 'examples/ip-intelligence')
    try {
        Write-Host "Running the IP Intelligence performance example..."
        go run ./performance -json-output $output
    } finally {
        Pop-Location
    }
    $results = Get-Content $output | ConvertFrom-Json -AsHashtable
    $higher.LookupsPerSecond = $results.HigherIsBetter.DetectionsPerSecond
    $lower.AvgMillisecsPerLookup = $results.LowerIsBetter.AvgMillisecsPerDetection
}

if (!$higher.Count -and !$lower.Count) {
    Write-Host "No engine affected, so no performance figures"
    exit
}

$resultsFile = Join-Path $rootDir "results_$Name.json"
@{ HigherIsBetter = $higher; LowerIsBetter = $lower } | ConvertTo-Json -Depth 3 |
    Set-Content $resultsFile
& "$rootDir/steps/publish-performance-results.ps1" -SourceFile $resultsFile -Name $Name -RepoName $RepoName
