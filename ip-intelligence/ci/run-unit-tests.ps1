$ErrorActionPreference = "Stop"
$PSNativeCommandUseErrorActionPreference = $true

$results = New-Item -ItemType directory -Force -Path "$PSScriptRoot/../test-results/unit"

$env:DATA_FILE = "$PWD/assets/51Degrees-EnterpriseIpiV41.ipi"

Push-Location "$PSScriptRoot/.."
try {
    go test -v ./ipi_interop ./ipi_onpremise 2>&1 | go-junit-report -set-exit-code -iocopy -out "$results/results.xml"
} finally {
    Pop-Location
}
