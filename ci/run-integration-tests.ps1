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

# The examples of each engine are packages of that engine's module.
if ($Products -contains 'device-detection') {
    $env:LICENSE_KEY = $DeviceDetection
    $env:DATA_FILE = "$PWD/assets/51Degrees-LiteV4.1.hash"
    $env:EVIDENCE_YAML = "$PWD/assets/20000 Evidence Records.yml"
    Test-Examples 'device-detection' (Join-Path (Get-ProductPath $RepoName 'device-detection') 'examples') 'device-detection'
}

if ($Products -contains 'device-detection' -or $Products -contains 'ip-intelligence') {
    $env:IPI_KEY = $IpIntelligence
    $env:IPI_DATA_FILE_URL = $IpIntelligenceUrl
    $env:DATA_FILE = "$PWD/assets/51Degrees-EnterpriseIpiV41.ipi"
    $env:EVIDENCE_YAML = "$PWD/assets/ip-intelligence-evidence.yml"
    # Used only by the mixed examples, which combine both products.
    $env:DD_DATA_FILE = "$PWD/assets/TAC-HashV41.hash"
    $ipi = Get-ProductPath $RepoName 'ip-intelligence'
    if ($Products -contains 'ip-intelligence') {
        Test-Examples 'ip-intelligence' (Join-Path $ipi 'examples') 'ip-intelligence'
    }
    # The mixed examples are their own Go module, so ./... above does not reach
    # them. They also use Device Detection, so a change to it runs them too.
    Test-Examples 'ip-intelligence' (Join-Path $ipi 'examples/mixed') 'mixed'
}

exit $failed
