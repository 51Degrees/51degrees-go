param (
    [string]$DeviceDetection,
    [string]$DeviceDetectionUrl,
    [string]$IpIntelligence,
    [string]$IpIntelligenceUrl,
    # The branch a pull request merges into, which its changes are measured
    # against.
    [string]$Branch = 'main'
)
$ErrorActionPreference = "Stop"

. $PSScriptRoot/products.ps1
$RepoPath = Join-Path $PSScriptRoot ".."
$Products = Get-AffectedProducts $RepoPath $Branch

# Device Detection: the Lite data file and the evidence the tests and examples
# use. IP Intelligence: the Enterprise data file and its evidence, and the
# Device Detection TAC file for the mixed examples, which combine both
# products.
$Assets = @()
if ($Products -contains 'device-detection') {
    $Assets += "51Degrees-LiteV4.1.hash", "20000 User Agents.csv", "20000 Evidence Records.yml"
}
if ($Products -contains 'device-detection' -or $Products -contains 'ip-intelligence') {
    $Assets += "51Degrees-EnterpriseIpiV41.ipi", "ip-intelligence-evidence.yml", "TAC-HashV41.hash"
}
if (!$Assets) {
    Write-Host "No assets needed"
    exit
}

./steps/fetch-assets.ps1 -Assets $Assets `
    -DeviceDetection:$DeviceDetection -DeviceDetectionUrl:$DeviceDetectionUrl `
    -IpIntelligence:$IpIntelligence -IpIntelligenceUrl:$IpIntelligenceUrl

# The IP Intelligence reload tests use the small ASN data file from the
# ip-intelligence-data submodule of cxx, and the Enterprise data file, which
# has different properties, and look for both in the module folder.
if ($Products -contains 'ip-intelligence') {
    $folder = Get-ProductPath $RepoPath 'ip-intelligence'
    $asn = "51Degrees-IPIV4AsnIpiV41.ipi"
    New-Item -ItemType SymbolicLink -Force `
        -Target (Join-Path $RepoPath "cxx/ip-intelligence-data/$asn") `
        -Path (Join-Path $folder $asn) | Out-Null
    $enterprise = "51Degrees-EnterpriseIpiV41.ipi"
    New-Item -ItemType SymbolicLink -Force -Target "$PWD/assets/$enterprise" `
        -Path (Join-Path $folder $enterprise) | Out-Null
}

# The Device Detection tests and examples look for their files in their own
# module folder.
if ($Products -contains 'device-detection') {
    foreach ($folder in (Get-ProductPath $RepoPath 'device-detection'), (Join-Path $RepoPath 'examples/device-detection')) {
        foreach ($asset in "51Degrees-LiteV4.1.hash", "20000 User Agents.csv", "20000 Evidence Records.yml") {
            New-Item -ItemType SymbolicLink -Force -Target "$PWD/assets/$asset" `
                -Path (Join-Path $folder $asset) | Out-Null
        }
    }
}
