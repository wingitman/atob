# install.ps1 — atob installer for Windows
#
# Usage (from PowerShell, run as your normal user — no admin required):
#   .\install.ps1
#   .\install.ps1 -InstallDir "C:\tools"
#   .\install.ps1 -InstallDir "$env:USERPROFILE\.local\bin"
#
# One-liner:
#   irm https://raw.githubusercontent.com/wingitman/atob/main/install.ps1 | iex
#
# The script will:
#   1. Check for an existing installation and report its version.
#   2. Build atob from source using the Go toolchain if available,
#      or download a pre-built binary from GitHub Releases.
#   3. Copy the binary to InstallDir.
#   4. Add InstallDir to the user PATH in the registry (no admin needed).
#   5. Print example commands.

#Requires -Version 5.1
[CmdletBinding()]
param(
    [string]$InstallDir = "$env:USERPROFILE\.local\bin"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$Repo      = 'wingitman/atob'
$Binary    = 'atob.exe'
$TmpDir    = Join-Path ([System.IO.Path]::GetTempPath()) ([System.IO.Path]::GetRandomFileName())

# ── helpers ────────────────────────────────────────────────────────────────────
function Write-Info    { param($msg) Write-Host "› $msg" -ForegroundColor Cyan }
function Write-Warn    { param($msg) Write-Host "! $msg" -ForegroundColor Yellow }
function Write-Success { param($msg) Write-Host "✓ $msg" -ForegroundColor Green }
function Write-Fail    { param($msg) Write-Host "✗ $msg" -ForegroundColor Red; exit 1 }

function Ensure-Dir {
    param([string]$Path)
    if (-not (Test-Path $Path)) {
        New-Item -ItemType Directory -Path $Path -Force | Out-Null
    }
}

# ── existing install check ─────────────────────────────────────────────────────
function Check-Existing {
    $cmd = Get-Command 'atob' -ErrorAction SilentlyContinue
    if ($cmd) {
        $ver = & atob --version 2>$null
        Write-Warn "atob is already installed: $ver"
        Write-Warn "Continuing will replace it."
        Write-Host ""
    }
}

# ── build from source ──────────────────────────────────────────────────────────
function Build-FromSource {
    Write-Info "Building atob from source…"

    $scriptDir = Split-Path -Parent $MyInvocation.PSCommandPath
    $srcDir    = $scriptDir

    if (-not (Test-Path (Join-Path $srcDir 'go.mod'))) {
        Write-Info "Cloning repository…"
        Ensure-Dir $TmpDir
        & git clone --depth=1 "https://github.com/$Repo.git" "$TmpDir\src"
        if ($LASTEXITCODE -ne 0) { Write-Fail "git clone failed." }
        $srcDir = "$TmpDir\src"
    }

    $version   = (& git -C $srcDir describe --tags --always --dirty 2>$null) ?? 'dev'
    $buildTime = (Get-Date -AsUTC -Format 'yyyy-MM-ddTHH:mm:ssZ')
    $outPath   = Join-Path $TmpDir 'atob.exe'

    Ensure-Dir $TmpDir
    Push-Location $srcDir
    try {
        & go build `
            -ldflags="-s -w -X main.version=$version -X main.buildTime=$buildTime" `
            -o $outPath `
            .
        if ($LASTEXITCODE -ne 0) { Write-Fail "go build failed." }
    } finally {
        Pop-Location
    }
    return $outPath
}

# ── download pre-built binary ──────────────────────────────────────────────────
function Download-Binary {
    Write-Info "Fetching latest release tag…"
    $apiUrl  = "https://api.github.com/repos/$Repo/releases/latest"
    $headers = @{ 'User-Agent' = 'atob-installer' }
    try {
        $release = Invoke-RestMethod -Uri $apiUrl -Headers $headers
    } catch {
        Write-Fail "Could not reach GitHub API. Check your internet connection."
    }
    $tag = $release.tag_name
    if (-not $tag) {
        Write-Fail "Could not determine latest release. Check https://github.com/$Repo/releases"
    }

    $filename = 'atob-windows-amd64.exe'
    $url      = "https://github.com/$Repo/releases/download/$tag/$filename"
    Write-Info "Downloading $filename ($tag)…"

    Ensure-Dir $TmpDir
    $outPath = Join-Path $TmpDir 'atob.exe'
    try {
        Invoke-WebRequest -Uri $url -OutFile $outPath -UseBasicParsing
    } catch {
        Write-Fail "Download failed. Verify release assets exist at:`n  $url"
    }
    return $outPath
}

# ── add directory to user PATH (registry, no admin) ───────────────────────────
function Ensure-InPath {
    param([string]$Dir)
    $regPath  = 'HKCU:\Environment'
    $current  = (Get-ItemProperty -Path $regPath -Name 'PATH' -ErrorAction SilentlyContinue).PATH ?? ''
    $entries  = $current -split ';' | Where-Object { $_ -ne '' }

    if ($entries -contains $Dir) { return }

    Write-Info "Adding $Dir to user PATH…"
    $newPath = ($entries + $Dir) -join ';'
    Set-ItemProperty -Path $regPath -Name 'PATH' -Value $newPath -Type ExpandString

    # Broadcast WM_SETTINGCHANGE so running processes pick up the new PATH.
    $signature = '[DllImport("user32.dll")] public static extern IntPtr SendMessageTimeout(IntPtr hWnd, uint Msg, UIntPtr wParam, string lParam, uint fuFlags, uint uTimeout, out UIntPtr lpdwResult);'
    $type = Add-Type -MemberDefinition $signature -Name 'Win32' -Namespace 'SetEnv' -PassThru
    $result = [UIntPtr]::Zero
    $type::SendMessageTimeout([IntPtr]0xffff, 0x1a, [UIntPtr]::Zero, 'Environment', 2, 5000, [ref]$result) | Out-Null

    $env:PATH = "$env:PATH;$Dir"
    Write-Success "$Dir added to PATH (takes effect in new terminals)."
}

# ── cleanup ────────────────────────────────────────────────────────────────────
function Remove-TmpDir {
    if (Test-Path $TmpDir) {
        Remove-Item -Path $TmpDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}

# ── main ───────────────────────────────────────────────────────────────────────
try {
    Write-Host ""
    Write-Host "atob installer" -ForegroundColor White
    Write-Host "────────────────────────────────"
    Write-Host ""

    Check-Existing

    Write-Info "Platform:    windows/amd64"
    Write-Info "Install dir: $InstallDir"
    Write-Host ""

    $binaryPath = ''
    if (Get-Command 'go' -ErrorAction SilentlyContinue) {
        Write-Info "Go toolchain found: $(& go version)"
        $binaryPath = Build-FromSource
    } else {
        Write-Warn "Go not found — downloading pre-built binary."
        Write-Warn "To build from source, install Go: https://go.dev/dl/"
        Write-Host ""
        $binaryPath = Download-Binary
    }

    Ensure-Dir $InstallDir
    $dest = Join-Path $InstallDir $Binary
    Copy-Item -Path $binaryPath -Destination $dest -Force
    Write-Success "atob installed → $dest"
    Write-Host ""

    Ensure-InPath $InstallDir

    # Version confirmation
    try {
        $ver = & $dest --version 2>$null
        Write-Success $ver
    } catch { }

    Write-Host ""
    Write-Host "Next steps:" -ForegroundColor White
    Write-Host ""
    Write-Host "  Interactive TUI (no arguments):"
    Write-Host "    atob"
    Write-Host ""
    Write-Host "  Open TUI with a file pre-loaded:"
    Write-Host "    atob .\myfile.json"
    Write-Host "    atob C:\Windows\System32\notepad.exe"
    Write-Host ""
    Write-Host "  CLI usage:"
    Write-Host '    echo hello world | atob base64'
    Write-Host '    atob {"a":1} yaml'
    Write-Host "    atob list"
    Write-Host ""
    $ConfigFile = Join-Path $env:APPDATA 'delbysoft\atob.toml'
    Write-Host "  Config file (created on first launch):"
    Write-Host "    $ConfigFile"
    Write-Host ""
    Write-Host "  Neovim plugin: https://github.com/wingitman/atob.nvim"
    Write-Host ""
} finally {
    Remove-TmpDir
}
