param (
    [string]$DeviceDetection,
    [string]$DeviceDetectionUrl,
    [string]$IpIntelligence,
    [string]$IpIntelligenceUrl
)
$ErrorActionPreference = "Stop"

. $PSScriptRoot/products.ps1
$RepoPath = Join-Path $PSScriptRoot ".."
# The prefetch in the Configure job fills the asset cache that every run that
# day restores, whatever it changes, so it fetches for every product.
$Products = $AllProducts
if ($env:GITHUB_JOB -ne 'Configure') {
    $Products = Get-AffectedProducts $RepoPath
}
$Data = Join-Path $RepoPath "cxx/data"

# Device Detection: the Lite data file and the evidence the tests and examples
# use. IP Intelligence: the Enterprise data file and its evidence, and the
# Device Detection TAC file for the mixed examples, which combine both
# products. Both: the free Lite data files the examples default to. IP
# Intelligence also needs the small Asn data file for its reload tests.
$Assets = @()
if ($Products -contains 'device-detection') {
    $Assets += "51Degrees-LiteV4.1.hash", "20000 User Agents.csv", "20000 Evidence Records.yml"
}
if ($Products -contains 'device-detection' -or $Products -contains 'ip-intelligence') {
    $Assets += "51Degrees-EnterpriseIpiV41.ipi", "ip-intelligence-evidence.yml", "TAC-HashV41.hash"
    $Assets += "51Degrees-LiteV4.1.hash", "51Degrees-LiteIpiV41.ipi"
}
if ($Products -contains 'ip-intelligence') {
    $Assets += "51Degrees-IPIV4AsnIpiV41.ipi"
}
$Assets = @($Assets | Select-Object -Unique)
if (!$Assets) {
    Write-Host "No assets needed"
    exit
}

# The assets are kept in ./assets, which the run caches once, so a test job
# restores them rather than downloading them.
./steps/fetch-assets.ps1 -Assets $Assets `
    -DeviceDetection:$DeviceDetection -DeviceDetectionUrl:$DeviceDetectionUrl `
    -IpIntelligence:$IpIntelligence -IpIntelligenceUrl:$IpIntelligenceUrl

# The examples default to the free Lite data files in cxx/data, and an example
# downloads a file when it is missing, so the assets are linked there.
if ($Products -contains 'device-detection' -or $Products -contains 'ip-intelligence') {
    New-Item -ItemType SymbolicLink -Force -Target "$PWD/assets/51Degrees-LiteV4.1.hash" `
        -Path (Join-Path $Data "device-detection/51Degrees-LiteV4.1.hash") | Out-Null
    New-Item -ItemType SymbolicLink -Force -Target "$PWD/assets/51Degrees-LiteIpiV41.ipi" `
        -Path (Join-Path $Data "ip-intelligence/51Degrees-LiteV41.ipi") | Out-Null
}

# The IP Intelligence reload tests use the small Asn data file and the
# Enterprise data file, which has different properties, and look for both in
# the module folder. The Asn file is copied into cxx/data, where it is found
# by the data repository's name.
if ($Products -contains 'ip-intelligence') {
    $folder = Get-ProductPath $RepoPath 'ip-intelligence'
    $asn = "51Degrees-IPIV4AsnIpiV41.ipi"
    Copy-Item "$PWD/assets/$asn" (Join-Path $Data "ip-intelligence/$asn")
    New-Item -ItemType SymbolicLink -Force `
        -Target (Join-Path $Data "ip-intelligence/$asn") `
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
