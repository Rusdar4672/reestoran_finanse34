$ErrorActionPreference = "Stop"
$env:GOCACHE = Join-Path $PSScriptRoot "..\.gocache"
$root = Resolve-Path (Join-Path $PSScriptRoot "..")
Set-Location $root

go test ./...
New-Item -ItemType Directory -Force -Path "build" | Out-Null
go build -buildvcs=false -o "build\RestaurantFinance.exe" ".\cmd\desktop"

Write-Host "Готово: build\RestaurantFinance.exe"
