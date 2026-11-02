param (
    [Parameter(Mandatory)][string]$IpIntelligence,
    [Parameter(Mandatory)][string]$IpIntelligenceUrl
)
$ErrorActionPreference = 'Stop'
$PSNativeCommandUseErrorActionPreference = $true

$results = New-Item -ItemType directory -Force -Path "$PSScriptRoot/../test-results/integration"

# IP Intelligence assets - used by all examples
$env:IPI_KEY = $IpIntelligence
$env:IPI_DATA_FILE_URL = $IpIntelligenceUrl
$env:DATA_FILE = "$PWD/assets/51Degrees-EnterpriseIpiV41.ipi"
$env:EVIDENCE_YAML = "$PWD/assets/ip-intelligence-evidence.yml"

# Device Detection asset - used only by the mixed examples (examples/mixed),
# which intentionally combine Device Detection and IP Intelligence
$env:DD_DATA_FILE = "$PWD/assets/TAC-HashV41.hash"

if ($IsMacOS) {
    # go-junit-report seems to count the harmless warning about -lm being
    # provided multiple times as an error
    $env:CGO_LDFLAGS = '-Wl,-no_warn_duplicate_libraries'
}

Push-Location "$PSScriptRoot/.."
try {
    go test -v ./examples/... 2>&1 | go-junit-report -set-exit-code -iocopy -out "$results/results.xml"
} finally {
    Pop-Location
}

# The mixed examples live in their own Go module (examples/mixed/go.mod), so
# they are not reached by ./examples/... above and must be tested explicitly.
Push-Location "$PSScriptRoot/../examples/mixed"
try {
    go test -v ./... 2>&1 | go-junit-report -set-exit-code -iocopy -out "$results/results-mixed.xml"
} finally {
    Pop-Location
}
