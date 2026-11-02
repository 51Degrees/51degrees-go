param (
    [string]$DeviceDetection,
    [string]$DeviceDetectionUrl,
    [string]$IpIntelligence,
    [string]$IpIntelligenceUrl
)
$ErrorActionPreference = "Stop"

. $PSScriptRoot/products.ps1
$RepoPath = Join-Path $PSScriptRoot ".."
$Products = Get-AffectedProducts $RepoPath

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
# ip-intelligence-data submodule of cxx, and look for it in the module folder.
if ($Products -contains 'ip-intelligence') {
    $asn = "51Degrees-IPIV4AsnIpiV41.ipi"
    New-Item -ItemType SymbolicLink -Force `
        -Target (Join-Path $RepoPath "cxx/ip-intelligence-data/$asn") `
        -Path (Join-Path (Get-ProductPath $RepoPath 'ip-intelligence') $asn) | Out-Null
}

# The Device Detection tests look for their files in the module folder.
if ($Products -contains 'device-detection') {
    foreach ($asset in "51Degrees-LiteV4.1.hash", "20000 User Agents.csv", "20000 Evidence Records.yml") {
        New-Item -ItemType SymbolicLink -Force -Target "$PWD/assets/$asset" `
            -Path (Join-Path (Get-ProductPath $RepoPath 'device-detection') $asset) | Out-Null
    }
}
