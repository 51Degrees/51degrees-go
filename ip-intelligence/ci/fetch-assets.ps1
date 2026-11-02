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
