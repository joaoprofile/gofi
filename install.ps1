<#
install.ps1 — install the gofi CLI on Windows.

Usage (PowerShell):
  iwr -useb https://raw.githubusercontent.com/joaoprofile/gofi-cli/main/install.ps1 | iex

Specific version:
  $env:GOFI_VERSION = "v0.2.0"
  iwr -useb https://raw.githubusercontent.com/joaoprofile/gofi-cli/main/install.ps1 | iex

The script installs into %LOCALAPPDATA%\Programs\gofi\bin and adds that
directory to the user PATH (no admin needed). Restart your shell for the
PATH update to take effect.
#>

$ErrorActionPreference = "Stop"

$Repo    = "joaoprofile/gofi-cli"
$Binary  = "gofi"
$Version = if ($env:GOFI_VERSION) { $env:GOFI_VERSION } else { "latest" }

$InstallDir = Join-Path $env:LOCALAPPDATA "Programs\gofi\bin"

function Write-Step($msg) { Write-Host "==> $msg" -ForegroundColor Cyan }
function Write-Warn($msg) { Write-Host "Warning: $msg" -ForegroundColor Yellow }
function Write-Err($msg)  { Write-Host "Error: $msg" -ForegroundColor Red; exit 1 }

# detect architecture (handles both native ARM64 and emulated x86 on ARM)
$archEnv = if ($env:PROCESSOR_ARCHITEW6432) { $env:PROCESSOR_ARCHITEW6432 } else { $env:PROCESSOR_ARCHITECTURE }
$arch = switch ($archEnv) {
    "AMD64" { "amd64" }
    "ARM64" { "arm64" }
    default { Write-Err "unsupported architecture: $archEnv" }
}

# enforce TLS 1.2 (older PowerShell defaults break GitHub API)
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

# resolve version
if ($Version -eq "latest") {
    Write-Step "resolving latest release"
    try {
        $latest = Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest" -UseBasicParsing
        $Version = $latest.tag_name
    } catch {
        Write-Err "could not resolve latest version: $($_.Exception.Message)"
    }
}

Write-Step "installing gofi $Version for windows/$arch"

$versionNoV = $Version.TrimStart("v")
$asset = "${Binary}_${versionNoV}_windows_${arch}.zip"
$assetUrl = "https://github.com/$Repo/releases/download/$Version/$asset"
$checksumsUrl = "https://github.com/$Repo/releases/download/$Version/checksums.txt"

# temp dir
$tmp = Join-Path $env:TEMP "gofi-install-$(Get-Random)"
New-Item -ItemType Directory -Path $tmp -Force | Out-Null

try {
    $assetPath = Join-Path $tmp $asset
    $checksumsPath = Join-Path $tmp "checksums.txt"

    Write-Step "downloading $asset"
    Invoke-WebRequest -Uri $assetUrl -OutFile $assetPath -UseBasicParsing

    Write-Step "downloading checksums.txt"
    Invoke-WebRequest -Uri $checksumsUrl -OutFile $checksumsPath -UseBasicParsing

    # verify sha256
    $line = Get-Content $checksumsPath | Where-Object { $_ -match "\s$([regex]::Escape($asset))$" } | Select-Object -First 1
    if (-not $line) { Write-Err "asset $asset not listed in checksums.txt" }
    $expected = ($line -split '\s+')[0].ToLower()
    $actual = (Get-FileHash $assetPath -Algorithm SHA256).Hash.ToLower()
    Write-Step "verifying sha256"
    if ($expected -ne $actual) {
        Write-Err "checksum mismatch (expected $expected, got $actual)"
    }

    # extract
    Write-Step "extracting"
    Expand-Archive -Path $assetPath -DestinationPath $tmp -Force
    $binarySrc = Join-Path $tmp "$Binary.exe"
    if (-not (Test-Path $binarySrc)) { Write-Err "binary $Binary.exe not found in archive" }

    # install
    if (-not (Test-Path $InstallDir)) {
        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    }
    $binaryDst = Join-Path $InstallDir "$Binary.exe"
    Move-Item -Path $binarySrc -Destination $binaryDst -Force
    Write-Step "installed to $binaryDst"

    # add to user PATH if missing
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $pathEntries = if ($userPath) { $userPath -split ';' } else { @() }
    if ($pathEntries -notcontains $InstallDir) {
        Write-Step "adding $InstallDir to user PATH"
        $newPath = if ($userPath) { "$userPath;$InstallDir" } else { $InstallDir }
        [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
        Write-Warn "Restart your shell to pick up the updated PATH."
    }

    Write-Step "done — run 'gofi h' to get started"
}
finally {
    Remove-Item -Path $tmp -Recurse -Force -ErrorAction SilentlyContinue
}
