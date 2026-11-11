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

# How long one attempt at downloading a data file may take. The download
# script's own limit covers only the wait for the server's first answer, so a
# transfer that stalls part way would otherwise hold the job until its own
# timeout.
$DownloadAttemptMinutes = 10
$DownloadAttempts = 2

# Downloads a data file with the script in the data submodule of cxx, in a
# process of its own so that an attempt can be stopped when it runs too long.
# A failed or stopped attempt is tried once more.
function Get-DataFile([string]$Name) {
    $script = Join-Path $RepoPath "cxx/data/get-data-files.ps1"
    $shell = (Get-Process -Id $PID).Path
    for ($attempt = 1; $attempt -le $DownloadAttempts; $attempt++) {
        $process = Start-Process -FilePath $shell -NoNewWindow -PassThru `
            -ArgumentList "-NoProfile -File `"$script`" $Name"
        # Reading the handle keeps the exit code once the process ends.
        $null = $process.Handle
        if ($process.WaitForExit($DownloadAttemptMinutes * 60000)) {
            if ($process.ExitCode -eq 0) {
                return
            }
            Write-Warning "Attempt $attempt to download $Name failed"
        } else {
            $process.Kill($true)
            $process.WaitForExit()
            Write-Warning "Attempt $attempt to download $Name took more than $DownloadAttemptMinutes minutes"
        }
    }
    throw "$Name was not downloaded"
}

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

# The examples default to the free Lite data files in cxx/data, and an example
# downloads a file when it is missing. Several example packages run at once
# and use the same files, so they are fetched once here, before any test runs,
# rather than by the examples while their tests run.
if ($Products -contains 'device-detection' -or $Products -contains 'ip-intelligence') {
    foreach ($lite in "51Degrees-LiteV4.1.hash", "51Degrees-LiteV41.ipi") {
        Get-DataFile $lite
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
    Get-DataFile $asn
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
