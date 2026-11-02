param (
    [Parameter(Mandatory=$true)]
    [string]$RepoName,
    [Parameter(Mandatory=$true)]
    [string]$OrgName,
    [bool]$DryRun = $false
)

Push-Location $RepoName
try {
    go get -u ./... || $(throw "'go get -u' failed")
    go mod tidy || $(throw "'go mod tidy' failed")

    Write-Output "Downloading latest device-detection-cxx..."
    Invoke-WebRequest "https://github.com/$OrgName/device-detection-cxx/archive/refs/heads/main.zip" -OutFile device-detection-cxx.zip

    Write-Output "Downloading latest common-cxx..."
    Invoke-WebRequest "https://github.com/$OrgName/common-cxx/archive/refs/heads/main.zip" -OutFile common-cxx.zip

    Write-Output "Extracting the archives..."
    Expand-Archive device-detection-cxx.zip -DestinationPath .
    Expand-Archive common-cxx.zip -DestinationPath .

    Write-Output "Combining repositories..."
    $commonCXX = New-Item -Force -Type Directory device-detection-cxx-main/src/common-cxx
    Move-Item -Force -Path common-cxx-main/* -Destination $commonCXX

    Write-Output "Generating amalgamation..."
    $src = "device-detection-cxx-main/src/"
    awk -f ci/amalgamate.awk $src/common-cxx/*c $src/*.c $src/hash/*.c >dd/device-detection-cxx.c
    awk -f ci/amalgamate.awk $src/hash/fiftyone.h $src/hash/hash.h $src/common-cxx/fiftyone.h >dd/device-detection-cxx.h
} finally {
    Write-Output "Cleaning up..."
    Remove-Item device-detection-cxx.zip, device-detection-cxx-main, common-cxx.zip, common-cxx-main -Recurse -Force
    Pop-Location
}
