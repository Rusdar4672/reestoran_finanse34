@echo off
setlocal
cd /d "%~dp0\.."
set "GOCACHE=%CD%\.gocache"
set "WINRES=%USERPROFILE%\go\bin\go-winres.exe"

if exist "%WINRES%" (
  "%WINRES%" simply --arch amd64 --manifest gui --icon build\windows\icon.ico --out cmd\desktop\restaurant-finance --product-name "Restaurant Finance" --file-description "Restaurant Finance" --original-filename "RestaurantFinance.exe" --product-version "1.0.0.0" --file-version "1.0.0.0"
  if errorlevel 1 exit /b 1
) else if not exist cmd\desktop\restaurant-finance_windows_amd64.syso (
  echo Windows resource compiler is missing.
  echo Install it once: go install github.com/tc-hib/go-winres@latest
  exit /b 1
)

go test -buildvcs=false ./...
if errorlevel 1 exit /b 1
if not exist build mkdir build
go build -buildvcs=false -trimpath -tags "desktop,production" -ldflags="-H windowsgui -s -w" -o build\RestaurantFinance.exe .\cmd\desktop
if errorlevel 1 exit /b 1
echo Ready: build\RestaurantFinance.exe
