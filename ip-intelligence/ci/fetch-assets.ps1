param (
    [string]$DeviceDetection,
    [string]$DeviceDetectionUrl,
    [string]$IpIntelligenceUrl
)
$ErrorActionPreference = "Stop"

# IP Intelligence assets: 51Degrees-EnterpriseIpiV41.ipi and
# ip-intelligence-evidence.yml.
# Device Detection asset: TAC-HashV41.hash - fetched only for the mixed
# examples (examples/mixed), which intentionally combine both products.
./steps/fetch-assets.ps1 -DeviceDetection:$DeviceDetection -DeviceDetectionUrl:$DeviceDetectionUrl -IpIntelligenceUrl:$IpIntelligenceUrl `
    -Assets "51Degrees-EnterpriseIpiV41.ipi", "ip-intelligence-evidence.yml", "TAC-HashV41.hash"

# The reload tests use the small ASN data file from ip-intelligence-data, and
# look for it in the repository root.
$asn = "51Degrees-IPIV4AsnIpiV41.ipi"
if (!(Test-Path "assets/$asn")) {
    Invoke-WebRequest "https://raw.githubusercontent.com/51Degrees/ip-intelligence-data/main/$asn" -OutFile "assets/$asn"
}
New-Item -ItemType SymbolicLink -Force -Target "$PWD/assets/$asn" -Path "$PSScriptRoot/../$asn"
