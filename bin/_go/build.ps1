# Rebuild the leveldb-writer.exe binary for Windows.
# Run from anywhere: .\bin\_go\build.ps1
param()
Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Error "Go not installed. Download from https://go.dev/dl/"
    exit 1
}

$HERE = $PSScriptRoot
$OUT = [System.IO.Path]::GetFullPath("$HERE\..\leveldb-writer.exe")

Push-Location "$HERE\leveldb_writer"
try {
    go build -trimpath -ldflags="-s -w" -o $OUT .
    Write-Host "built: $OUT"
} finally {
    Pop-Location
}
