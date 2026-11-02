$ErrorActionPreference = "Stop"

./go/setup-environment.ps1

# The examples require the engines at versions that may not be published yet.
& $PSScriptRoot/create-workspace.ps1
