# Codg installer for Windows (PowerShell) -- downloads the codg CLI from GitHub releases.
#
# Usage:
#   irm https://raw.githubusercontent.com/vcaesar/codg/main/demo/boot.ps1 | iex
#
#   # Specific version:
#   & ([scriptblock]::Create((irm https://raw.githubusercontent.com/vcaesar/codg/main/demo/boot.ps1))) -Version 2.0.2
#
#   # Local binary:
#   .\boot.ps1 -Binary C:\path\to\codg.exe
#
#   # Skip PATH modification:
#   .\boot.ps1 -NoModifyPath

[CmdletBinding()]
param(
    [Alias('v')]
    [string]$Version = $env:VERSION,

    [Alias('b')]
    [string]$Binary = '',

    [switch]$NoModifyPath
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$App  = 'codg'
$Repo = 'vcaesar/codg'

function Write-Muted  { param([string]$Msg) Write-Host $Msg -ForegroundColor DarkGray }
function Write-Info   { param([string]$Msg) Write-Host $Msg -ForegroundColor Cyan }
function Write-WarnC  { param([string]$Msg) Write-Host $Msg -ForegroundColor Yellow }
function Write-ErrorC { param([string]$Msg) Write-Host $Msg -ForegroundColor Red }

function Get-Target {
    $os = 'windows'

    $rawArch = $env:PROCESSOR_ARCHITECTURE
    if ($env:PROCESSOR_ARCHITEW6432) { $rawArch = $env:PROCESSOR_ARCHITEW6432 }

    switch -regex ($rawArch) {
        '^(AMD64|x86_64)$'   { $arch = 'amd64'; break }
        '^(ARM64|aarch64)$'  { $arch = 'arm64'; break }
        default {
            Write-ErrorC "Unsupported architecture: $rawArch"
            exit 1
        }
    }

    [pscustomobject]@{ OS = $os; Arch = $arch }
}

function Get-LatestVersion {
    try {
        $resp = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" -UseBasicParsing
    } catch {
        Write-ErrorC "Failed to fetch latest version information: $($_.Exception.Message)"
        exit 1
    }
    $tag = $resp.tag_name
    if (-not $tag) {
        Write-ErrorC 'Failed to fetch latest version information'
        exit 1
    }
    return ($tag -replace '^v', '')
}

function Test-InstalledVersion {
    param([string]$Wanted)

    $cmd = Get-Command $App -ErrorAction SilentlyContinue
    if (-not $cmd) { return }

    try {
        $installedRaw = (& $cmd.Source --version 2>$null) -join ' '
    } catch { return }

    if (-not $installedRaw) { return }

    $installed = ($installedRaw.Trim().Split(' ')[-1]) -replace '^v', ''
    if ($installed -eq $Wanted) {
        Write-Muted "Version $Wanted already installed"
        exit 0
    }
    if ($installed) {
        Write-Muted "Installed version: $installed -> upgrading to $Wanted"
    }
}

function Install-FromRelease {
    $target = Get-Target

    $requested = $Version
    if ([string]::IsNullOrWhiteSpace($requested)) {
        $requested = Get-LatestVersion
    }
    $requested = $requested -replace '^v', ''

    Test-InstalledVersion -Wanted $requested

    $filename = "${App}_$($target.OS)_$($target.Arch).zip"
    $url      = "https://github.com/$Repo/releases/download/v$requested/$filename"
    $tagUrl   = "https://github.com/$Repo/releases/tag/v$requested"

    try {
        $head = Invoke-WebRequest -Uri $tagUrl -Method Head -UseBasicParsing -ErrorAction Stop
        $status = [int]$head.StatusCode
    } catch {
        $status = 0
        if ($_.Exception.Response) {
            $status = [int]$_.Exception.Response.StatusCode
        }
    }
    if ($status -eq 404) {
        Write-ErrorC "Error: Release v$requested not found"
        Write-Muted  "Available releases: https://github.com/$Repo/releases"
        exit 1
    }

    Write-Host ""
    Write-Muted "Installing $App version: $requested ($($target.OS)/$($target.Arch))"

    $tmpDir = Join-Path ([System.IO.Path]::GetTempPath()) ("${App}_install_" + [System.Guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $tmpDir -Force | Out-Null
    try {
        $zipPath = Join-Path $tmpDir $filename
        Invoke-WebRequest -Uri $url -OutFile $zipPath -UseBasicParsing

        $extractDir = Join-Path $tmpDir 'extracted'
        New-Item -ItemType Directory -Path $extractDir -Force | Out-Null
        Expand-Archive -Path $zipPath -DestinationPath $extractDir -Force

        $binName = "$App.exe"
        $src = Get-ChildItem -Path $extractDir -Recurse -Filter $binName -File |
               Select-Object -First 1
        if (-not $src) {
            Write-ErrorC "Error: '$binName' not found in downloaded archive"
            exit 1
        }

        if (-not (Test-Path $script:InstallDir)) {
            New-Item -ItemType Directory -Path $script:InstallDir -Force | Out-Null
        }
        $dest = Join-Path $script:InstallDir $binName
        Copy-Item -Path $src.FullName -Destination $dest -Force
    } finally {
        Remove-Item -Path $tmpDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}

function Install-FromBinary {
    if (-not (Test-Path $Binary -PathType Leaf)) {
        Write-ErrorC "Error: Binary not found at $Binary"
        exit 1
    }
    Write-Host ""
    Write-Muted "Installing $App from: $Binary"
    if (-not (Test-Path $script:InstallDir)) {
        New-Item -ItemType Directory -Path $script:InstallDir -Force | Out-Null
    }
    $dest = Join-Path $script:InstallDir "$App.exe"
    Copy-Item -Path $Binary -Destination $dest -Force
}

function Add-ToUserPath {
    param([string]$Dir)

    if ($NoModifyPath) { return }

    $current = [Environment]::GetEnvironmentVariable('Path', 'User')
    if ($null -eq $current) { $current = '' }

    $parts = $current.Split(';', [StringSplitOptions]::RemoveEmptyEntries)
    $already = $false
    foreach ($p in $parts) {
        if ($p.TrimEnd('\') -ieq $Dir.TrimEnd('\')) { $already = $true; break }
    }

    if ($already) {
        Write-Muted "Command already exists in user PATH, skipping."
    } else {
        $newPath = if ([string]::IsNullOrEmpty($current)) { $Dir } else { "$Dir;$current" }
        try {
            [Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
            Write-Muted "Added $App to `$PATH (user scope)"
        } catch {
            Write-WarnC "Could not update user PATH automatically. Add this manually:"
            Write-Host  "  setx PATH `"$Dir;%PATH%`""
            return
        }
    }

    # Also update current session so `codg` works without relaunch.
    if (-not (($env:Path -split ';') -contains $Dir)) {
        $env:Path = "$Dir;$env:Path"
    }
}

# --- main ---------------------------------------------------------------

$script:InstallDir = Join-Path $HOME ".$App\bin"
if (-not (Test-Path $script:InstallDir)) {
    New-Item -ItemType Directory -Path $script:InstallDir -Force | Out-Null
}

# Prefer TLS 1.2+ on older PowerShell.
try {
    [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
} catch { }

if (-not [string]::IsNullOrWhiteSpace($Binary)) {
    Install-FromBinary
} else {
    Install-FromRelease
}

Add-ToUserPath -Dir $script:InstallDir

# GitHub Actions integration.
if ($env:GITHUB_ACTIONS -eq 'true' -and $env:GITHUB_PATH) {
    Add-Content -Path $env:GITHUB_PATH -Value $script:InstallDir
    Write-Muted "Added $script:InstallDir to `$GITHUB_PATH"
}

@'

                    #
 ####  ####  ###  ####
 #  #  #  #  # #  #  
 ####  ####  #  # ####

'@ | Write-Host

Write-Host ("Codg installed to: {0}\{1}.exe" -f $script:InstallDir, $App)
Write-Host ""
Write-Host "  cd <project>  # open a project"
Write-Host "  $App          # run codg"
Write-Host ""
Write-Muted "More info: https://github.com/$Repo"
Write-Host ""
