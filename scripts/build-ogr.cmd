@echo off
setlocal
cd /d "%~dp0\.."
set "GOCACHE=%CD%\.gocache"

go test -buildvcs=false ./...
if errorlevel 1 exit /b 1
if not exist build mkdir build
go build -buildvcs=false -trimpath -tags "desktop,production,ogr" -ldflags="-H windowsgui -s -w" -o build\RestaurantFinanceOGR.exe .\cmd\desktop
if errorlevel 1 exit /b 1
echo Ready: build\RestaurantFinanceOGR.exe
