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
    # Absolute, because the example runs from the module folder.
    $output = Join-Path $rootDir "summary-device-detection.json"
    Remove-Item $output -ErrorAction SilentlyContinue
    Push-Location (Get-ProductPath $RepoName 'device-detection')
    try {
        Write-Host "Running the Device Detection performance example..."
        go run ./examples/dd/performance -json-output $output
    } finally {
        Pop-Location
    }
    $results = Get-Content $output | ConvertFrom-Json -AsHashtable
    $higher.DetectionsPerSecond = $results.HigherIsBetter.DetectionsPerSecond
    $lower.AvgMillisecsPerDetection = $results.LowerIsBetter.AvgMillisecsPerDetection
}

if ($Products -contains 'ip-intelligence') {
    $env:DATA_FILE = "$rootDir/assets/51Degrees-EnterpriseIpiV41.ipi"
    Push-Location (Get-ProductPath $RepoName 'ip-intelligence')
    try {
        Write-Host "Running the IP Intelligence performance example..."
        go run ./examples/performance
        # The example writes its figures to a report.
        switch -File performance_report.log -Regex {
            'Average ([^ ]+) ms per' { $lower.AvgMillisecsPerLookup = [double]$matches.1 }
            'Average ([^ ]+) detections per second' { $higher.LookupsPerSecond = [double]$matches.1 }
        }
        if (!$higher.LookupsPerSecond -or !$lower.AvgMillisecsPerLookup) {
            Get-Content performance_report.log | Write-Error
        }
    } finally {
        Pop-Location
    }
}

if (!$higher.Count -and !$lower.Count) {
    Write-Host "No engine affected, so no performance figures"
    exit
}

$resultsFile = Join-Path $rootDir "results_$Name.json"
@{ HigherIsBetter = $higher; LowerIsBetter = $lower } | ConvertTo-Json -Depth 3 |
    Set-Content $resultsFile
& "$rootDir/steps/publish-performance-results.ps1" -SourceFile $resultsFile -Name $Name -RepoName $RepoName
