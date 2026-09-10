param(
    [Parameter(Mandatory = $true)]
    [string]$AccessKey,

    [Parameter(Mandatory = $true)]
    [string]$SecretKey,

    [Parameter(Mandatory = $true)]
    [string]$ServerHttpAddr,

    [Parameter(Mandatory = $true)]
    [string]$ServerEdgeAddr,

    [switch]$Help
)

$ErrorActionPreference = "Stop"

function Show-Help {
    Write-Host "Usage: install.ps1 -AccessKey xxx -SecretKey yyy -ServerHttpAddr URL-or-host:port -ServerEdgeAddr host:port"
    exit 0
}

if ($Help) { Show-Help }

Write-Host "Starting Liaison Edge installation..." -ForegroundColor Green
Write-Host "OS: windows-amd64" -ForegroundColor Green

# ---------------- Paths ----------------
$InstallDir = "C:\Program Files\Liaison"
$BinDir     = Join-Path $InstallDir "bin"
$ConfDir    = Join-Path $InstallDir "conf"
$LogDir     = Join-Path $InstallDir "logs"

$BinaryName = "liaison-edge.exe"

# ---------------- Temp ----------------
$TempDir = Join-Path ([System.IO.Path]::GetTempPath()) "liaison-edge-install"
if (Test-Path $TempDir) {
    Remove-Item $TempDir -Recurse -Force -ErrorAction SilentlyContinue
}
New-Item -ItemType Directory -Path $TempDir | Out-Null

# ---------------- Download ----------------
$PackageName = "liaison-edge-windows-amd64.tar.gz"
$DownloadBase = $ServerHttpAddr.TrimEnd('/')
if ($DownloadBase -notmatch '^https?://') { $DownloadBase = "https://$DownloadBase" }
$PackageUrl  = "$DownloadBase/packages/edge/$PackageName"
$PackagePath = Join-Path $TempDir $PackageName

Write-Host "Downloading package..." -ForegroundColor Yellow
Write-Host "URL: $PackageUrl" -ForegroundColor Yellow

# 强制 TLS1.2（Windows 旧环境）
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$downloaded = $false

# 优先 curl（最稳）
if (Get-Command curl.exe -ErrorAction SilentlyContinue) {
    try {
        curl.exe -f -L $PackageUrl -o $PackagePath
        $downloaded = $true
    } catch {}
}

# fallback: Invoke-WebRequest
if (-not $downloaded) {
    Invoke-WebRequest -Uri $PackageUrl -OutFile $PackagePath -UseBasicParsing
}

if (-not (Test-Path $PackagePath)) {
    Write-Host "ERROR: download failed" -ForegroundColor Red
    exit 1
}

if ((Get-Item $PackagePath).Length -le 0) {
    Write-Host "ERROR: downloaded file is empty" -ForegroundColor Red
    exit 1
}

Write-Host "Download OK" -ForegroundColor Green

# ---------------- Extract ----------------
Write-Host "Extracting..." -ForegroundColor Yellow

if (-not (Get-Command tar -ErrorAction SilentlyContinue)) {
    Write-Host "ERROR: tar not found (need Windows 10 1803+ or Git for Windows)" -ForegroundColor Red
    exit 1
}

Push-Location $TempDir
tar -xzf $PackageName
Pop-Location

$BinaryPath = Join-Path $TempDir $BinaryName
if (-not (Test-Path $BinaryPath)) {
    Write-Host "ERROR: binary not found after extract" -ForegroundColor Red
    exit 1
}

# ---------------- Install ----------------
Write-Host "Installing..." -ForegroundColor Yellow

New-Item -ItemType Directory -Force -Path $BinDir, $ConfDir, $LogDir | Out-Null
Copy-Item $BinaryPath (Join-Path $BinDir $BinaryName) -Force

# ---------------- Config ----------------
$ConfigFile = Join-Path $ConfDir "liaison-edge.yaml"

$configLines = @(
"manager:",
"  dial:",
"    addrs:",
"      - $ServerEdgeAddr",
"    network: tcp",
"    tls:",
"      enable: true",
"      insecure_skip_verify: true",
"  auth:",
"    access_key: `"$AccessKey`"",
"    secret_key: `"$SecretKey`"",
"log:",
"  level: info",
"  file: $($LogDir -replace '\\','/')/liaison-edge.log",
"  maxsize: 100",
"  maxrolls: 10"
)

$configLines -join "`n" | Set-Content -Path $ConfigFile -Encoding UTF8

Write-Host "Config written: $ConfigFile" -ForegroundColor Green

# ---------------- Run ----------------
Write-Host ""
Write-Host "Choose run mode:"
Write-Host "1) Run in background (default)"
Write-Host "2) Skip"

$choice = Read-Host "Select [1-2]"
if ([string]::IsNullOrWhiteSpace($choice)) { $choice = "1" }

if ($choice -eq "1") {
    Write-Host "Starting Edge..." -ForegroundColor Yellow
    $arg = "-c `"$ConfigFile`""
    Start-Process `
        -FilePath (Join-Path $BinDir $BinaryName) `
        -ArgumentList $arg `
        -WindowStyle Hidden `
        -RedirectStandardOutput (Join-Path $LogDir "liaison-edge.log") `
        -RedirectStandardError  (Join-Path $LogDir "liaison-edge.err.log")

    Write-Host "Edge started in background" -ForegroundColor Green
}

# ---------------- Cleanup ----------------
Remove-Item $TempDir -Recurse -Force -ErrorAction SilentlyContinue

Write-Host ""
Write-Host "Liaison Edge installation completed successfully!" -ForegroundColor Green
