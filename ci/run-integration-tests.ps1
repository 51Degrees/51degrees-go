param (
    [Parameter(Mandatory)][string]$RepoName,
    [Parameter(Mandatory)][string]$Name,
    [string]$DeviceDetection,
    [string]$IpIntelligence,
    [string]$IpIntelligenceUrl
)
$ErrorActionPreference = "Stop"

. $PSScriptRoot/products.ps1

$Results = Join-Path $RepoName "test-results/integration"
$Products = Get-AffectedProducts $RepoName
$failed = 0

function Test-Examples([string]$Product, [string]$ModulePath, [string]$JUnitName) {
    $code = Invoke-GoTests -ModulePath $ModulePath -Packages @('./...') `
        -JUnitPath "$Results/$Name-$JUnitName.xml"
    if ($code -ne 0) {
        Write-Host "Integration tests for '$JUnitName' failed with exit code $code"
        $script:failed = $code
    }
}

# The examples of each engine are a module of their own.
if ($Products -contains 'device-detection') {
    $env:LICENSE_KEY = $DeviceDetection
    $env:DATA_FILE = "$PWD/assets/51Degrees-LiteV4.1.hash"
    $env:EVIDENCE_YAML = "$PWD/assets/20000 Evidence Records.yml"
    Test-Examples 'device-detection' (Join-Path $RepoName 'examples/device-detection') 'device-detection'
}

if ($Products -contains 'device-detection' -or $Products -contains 'ip-intelligence') {
    $env:IPI_KEY = $IpIntelligence
    $env:IPI_DATA_FILE_URL = $IpIntelligenceUrl
    $env:DATA_FILE = "$PWD/assets/51Degrees-EnterpriseIpiV41.ipi"
    $env:EVIDENCE_YAML = "$PWD/assets/ip-intelligence-evidence.yml"
    # Used only by the mixed examples, which combine both products.
    $env:DD_DATA_FILE = "$PWD/assets/TAC-HashV41.hash"
    if ($Products -contains 'ip-intelligence') {
        Test-Examples 'ip-intelligence' (Join-Path $RepoName 'examples/ip-intelligence') 'ip-intelligence'
    }
    # The mixed examples use both engines, so a change to either runs them.
    Test-Examples 'ip-intelligence' (Join-Path $RepoName 'examples/mixed') 'mixed'
}

exit $failed
