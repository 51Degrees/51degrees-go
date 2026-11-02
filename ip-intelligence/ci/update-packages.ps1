param (
    [Parameter(Mandatory)][string]$RepoName,
    [Parameter(Mandatory)][string]$OrgName,
    [bool]$DryRun = $false
)
$ErrorActionPreference = "Stop"
$PSNativeCommandUseErrorActionPreference = $true

Push-Location $RepoName
try {
    go get -u ./...
    go mod tidy

    # The mixed examples are a separate Go module (it carries the
    # device-detection-go dependency), so update it explicitly
    Push-Location examples/mixed
    try {
        go get -u ./...
        go mod tidy
    } finally {
        Pop-Location
    }

    Write-Output "Cloning latest ip-intelligence-cxx..."
    git clone --depth=1 --recurse-submodules --shallow-submodules "https://github.com/$OrgName/ip-intelligence-cxx.git"

    Write-Output "Generating amalgamation..."
    $src = "ip-intelligence-cxx/src"
    awk -f ci/amalgamate.awk $src/fiftyone.h $src/ip-graph-cxx/graph.h >ipi_interop/ip-intelligence-cxx.h
    awk -f ci/amalgamate.awk $src/common-cxx/*.c $src/ip-graph-cxx/*.c $src/*.c >ipi_interop/ip-intelligence-cxx.c

    # The amalgamated C sources inline 51degrees.com documentation links that
    # upstream repositories (e.g. common-cxx) tag with their own utm_campaign.
    # The UTM lint requires every link to carry this repository's campaign, so
    # rewrite the campaign in the generated files to match the repo name.
    $campaign = $RepoName.ToLowerInvariant()
    Write-Output "Setting utm_campaign=$campaign in amalgamated files..."
    foreach ($f in "ipi_interop/ip-intelligence-cxx.h", "ipi_interop/ip-intelligence-cxx.c") {
        $content = (Get-Content -Raw $f) -replace 'utm_campaign=[A-Za-z0-9._-]+', "utm_campaign=$campaign"
        Set-Content -Path $f -Value $content -NoNewline
    }
} finally {
    Write-Output "Cleaning up..."
    Remove-Item -Recurse -Force ip-intelligence-cxx
    Pop-Location
}
