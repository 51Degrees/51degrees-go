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

# The IP Intelligence and mixed examples default to the free Lite data file,
# and an example downloads it when it is missing. The examples of both modules
# use the same file in cxx/data, so it is fetched once here, before any test
# runs, rather than by an example while its tests run.
if ($Products -contains 'device-detection' -or $Products -contains 'ip-intelligence') {
    $data = Join-Path $RepoPath "cxx/data"
    $lite = "51Degrees-LiteV41.ipi"
    $global:LASTEXITCODE = 0
    & (Join-Path $data "get-data-files.ps1") $lite
    if ($LASTEXITCODE -ne 0) {
        throw "$lite was not downloaded"
    }
}

# The IP Intelligence reload tests use the small Asn data file and the
# Enterprise data file, which has different properties, and look for both in
# the module folder. The script in the data submodule of cxx downloads the Asn
# file.
if ($Products -contains 'ip-intelligence') {
    $folder = Get-ProductPath $RepoPath 'ip-intelligence'
    $data = Join-Path $RepoPath "cxx/data"
    $asn = "51Degrees-IPIV4AsnIpiV41.ipi"
    $global:LASTEXITCODE = 0
    & (Join-Path $data "get-data-files.ps1") $asn
    if ($LASTEXITCODE -ne 0) {
        throw "$asn was not downloaded"
    }
    New-Item -ItemType SymbolicLink -Force `
        -Target (Join-Path $data "ip-intelligence/$asn") `
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
