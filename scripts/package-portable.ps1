param(
    [Parameter(Mandatory = $true)]
    [string]$WebView2Runtime
)

$ErrorActionPreference = "Stop"
$projectRoot = Split-Path -Parent $PSScriptRoot
$runtimeSource = (Resolve-Path -LiteralPath $WebView2Runtime).Path
$runtimeExecutable = Join-Path $runtimeSource "msedgewebview2.exe"

if (-not (Test-Path -LiteralPath $runtimeExecutable -PathType Leaf)) {
    throw "The selected folder is not a Fixed Version WebView2 Runtime: msedgewebview2.exe was not found."
}

& (Join-Path $PSScriptRoot "build.cmd")
if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
}

$packageRoot = Join-Path $projectRoot "build\portable"
$runtimeTarget = Join-Path $packageRoot "WebView2Runtime"

if (Test-Path -LiteralPath $packageRoot) {
    $resolvedPackage = (Resolve-Path -LiteralPath $packageRoot).Path
    $expectedRoot = [System.IO.Path]::GetFullPath((Join-Path $projectRoot "build"))
    if (-not $resolvedPackage.StartsWith($expectedRoot, [System.StringComparison]::OrdinalIgnoreCase)) {
        throw "Refusing to replace a package directory outside build."
    }
    Remove-Item -LiteralPath $packageRoot -Recurse -Force
}

New-Item -ItemType Directory -Path $runtimeTarget -Force | Out-Null
Copy-Item -LiteralPath (Join-Path $projectRoot "build\RestaurantFinance.exe") -Destination $packageRoot
Copy-Item -Path (Join-Path $runtimeSource "*") -Destination $runtimeTarget -Recurse -Force

Write-Host "Portable package ready: $packageRoot"
