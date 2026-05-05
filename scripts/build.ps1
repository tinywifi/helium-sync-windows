$ErrorActionPreference = "Stop"

$Root = (Resolve-Path "$PSScriptRoot\..").Path
$Out = Join-Path $Root "bin\helium-sync.exe"

go build -o $Out ./cmd/helium-sync
Write-Host "built $Out"
