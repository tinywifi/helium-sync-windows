@echo off
setlocal
set "HERE=%~dp0"
if exist "%HERE%helium-sync.exe" (
  "%HERE%helium-sync.exe" %*
) else (
  go run "%HERE%..\cmd\helium-sync" %*
)
