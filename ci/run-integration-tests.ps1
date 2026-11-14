param (
    [Parameter(Mandatory)][string]$RepoName,
    [Parameter(Mandatory)][string]$Name,
    [string]$DeviceDetection,
    [string]$IpIntelligence,
    [string]$IpIntelligenceUrl,
    # The branch a pull request merges into, which its changes are measured
    # against.
    [string]$Branch = 'main'
)
$ErrorActionPreference = "Stop"

. $PSScriptRoot/products.ps1

$Results = Join-Path $RepoName "test-results/integration"
$Products = Get-AffectedProducts $RepoName $Branch
$failed = 0

function Test-Examples([string]$Product, [string]$ModulePath, [string]$JUnitName) {
    $code = Invoke-GoTests -ModulePath $ModulePath -Packages @('./...') `
        -JUnitPath "$Results/$Name-$JUnitName.xml"
    if ($code -ne 0) {
        Write-Host "Integration tests for '$JUnitName' failed with exit code $code"
        $script:failed = $code
    }
}

# The examples of each engine are a module of their own. Each block sets the
# licence key of the product it runs, because the examples of both read the one
# name, and a block's run happens before the next block sets it again.
if ($Products -contains 'device-detection') {
    $env:_51DEGREES_LICENSE_KEY = $DeviceDetection
    $env:_51DEGREES_DD_PATH = "$PWD/assets/51Degrees-LiteV4.1.hash"
    $env:_51DEGREES_EVIDENCE_YAML = "$PWD/assets/20000 Evidence Records.yml"
    Test-Examples 'device-detection' (Join-Path $RepoName 'examples/device-detection') 'device-detection'
}

if ($Products -contains 'device-detection' -or $Products -contains 'ip-intelligence') {
    $env:_51DEGREES_LICENSE_KEY = $IpIntelligence
    $env:_51DEGREES_IPI_DATA_FILE_URL = $IpIntelligenceUrl
    $env:_51DEGREES_IPI_PATH = "$PWD/assets/51Degrees-EnterpriseIpiV41.ipi"
    $env:_51DEGREES_EVIDENCE_YAML = "$PWD/assets/ip-intelligence-evidence.yml"
    # Used only by the mixed examples, which combine both products.
    $env:_51DEGREES_DD_PATH = "$PWD/assets/TAC-HashV41.hash"
    if ($Products -contains 'ip-intelligence') {
        Test-Examples 'ip-intelligence' (Join-Path $RepoName 'examples/ip-intelligence') 'ip-intelligence'
    }
    # The mixed examples use both engines, so a change to either runs them.
    Test-Examples 'ip-intelligence' (Join-Path $RepoName 'examples/mixed') 'mixed'
}

# Building the Pipeline examples checks that each one compiles with the
# Pipeline module, and their test runs each one and checks how it ends.
if ($Products -contains 'pipeline') {
    $examples = Join-Path $RepoName 'examples/pipeline'
    Push-Location $examples
    try {
        Write-Host "Building the Pipeline examples"
        go build ./...
        if ($LASTEXITCODE -ne 0) {
            Write-Host "Building the Pipeline examples failed with exit code $LASTEXITCODE"
            $failed = $LASTEXITCODE
        }
    } finally {
        Pop-Location
    }
    Test-Examples 'pipeline' $examples 'pipeline'
}

exit $failed
