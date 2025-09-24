# DeeSpec installer for Windows
# Usage: iwr -useb https://raw.githubusercontent.com/YoshitsuguKoike/deespec/main/scripts/install.ps1 | iex

$ErrorActionPreference = 'Stop'

$RepoOwner = "YoshitsuguKoike"
$RepoName = "deespec"
$BinName = "deespec"

function Write-ColorOutput {
    param(
        [string]$Message,
        [string]$Type = "INFO"
    )

    switch ($Type) {
        "INFO" { Write-Host "[INFO] " -ForegroundColor Green -NoNewline }
        "WARN" { Write-Host "[WARN] " -ForegroundColor Yellow -NoNewline }
        "ERROR" { Write-Host "[ERROR] " -ForegroundColor Red -NoNewline }
    }
    Write-Host $Message
}

function Get-LatestRelease {
    Write-ColorOutput "Fetching latest release information..."

    try {
        $ApiUrl = "https://api.github.com/repos/$RepoOwner/$RepoName/releases/latest"
        $Response = Invoke-RestMethod -Uri $ApiUrl -Method Get
        $Tag = $Response.tag_name

        if (-not $Tag) {
            throw "Could not determine latest release tag"
        }

        Write-ColorOutput "Latest version: $Tag"
        return $Tag
    }
    catch {
        Write-ColorOutput "Failed to fetch latest release: $_" -Type "ERROR"
        exit 1
    }
}

function Get-SystemArchitecture {
    $Arch = [System.Environment]::Is64BitOperatingSystem

    if ($Arch) {
        return "amd64"
    } else {
        Write-ColorOutput "32-bit Windows is not supported" -Type "ERROR"
        exit 1
    }
}

function Install-DeeSpec {
    param(
        [string]$Tag
    )

    $Arch = Get-SystemArchitecture
    $AssetName = "${BinName}_windows_${Arch}.exe"
    $DownloadUrl = "https://github.com/$RepoOwner/$RepoName/releases/download/$Tag/$AssetName"

    # Determine installation directory
    $InstallDir = "$env:USERPROFILE\AppData\Local\Microsoft\WindowsApps"
    $DestPath = Join-Path $InstallDir "$BinName.exe"

    # Create directory if it doesn't exist
    if (-not (Test-Path $InstallDir)) {
        Write-ColorOutput "Creating installation directory: $InstallDir"
        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    }

    # Download binary
    Write-ColorOutput "Downloading $AssetName..."
    $TempFile = [System.IO.Path]::GetTempFileName()

    try {
        Invoke-WebRequest -Uri $DownloadUrl -OutFile $TempFile -UseBasicParsing
    }
    catch {
        Write-ColorOutput "Failed to download from $DownloadUrl" -Type "ERROR"
        Write-ColorOutput "Error: $_" -Type "ERROR"
        Remove-Item $TempFile -Force -ErrorAction SilentlyContinue
        exit 1
    }

    # Check if existing installation exists
    if (Test-Path $DestPath) {
        Write-ColorOutput "Existing $BinName found, replacing..." -Type "WARN"
        Remove-Item $DestPath -Force
    }

    # Move to final destination
    try {
        Move-Item -Path $TempFile -Destination $DestPath -Force
        Write-ColorOutput "✅ Successfully installed $BinName to $DestPath"
    }
    catch {
        Write-ColorOutput "Failed to install to $DestPath" -Type "ERROR"
        Write-ColorOutput "Error: $_" -Type "ERROR"
        Remove-Item $TempFile -Force -ErrorAction SilentlyContinue
        exit 1
    }

    # Verify PATH contains WindowsApps
    $PathArray = $env:PATH.Split(';')
    $WindowsAppsInPath = $PathArray | Where-Object { $_ -like "*WindowsApps*" }

    if (-not $WindowsAppsInPath) {
        Write-ColorOutput "WindowsApps directory is not in PATH" -Type "WARN"
        Write-ColorOutput "You may need to add it manually to your system PATH" -Type "WARN"
        Write-ColorOutput "Directory: $InstallDir" -Type "WARN"
    }

    return $DestPath
}

function Test-Installation {
    param(
        [string]$ExePath
    )

    Write-ColorOutput "`nVerifying installation..."

    # Try direct path first
    if (Test-Path $ExePath) {
        try {
            $Version = & $ExePath --version 2>$null
            if ($Version) {
                Write-ColorOutput "Version: $Version"
            }
        }
        catch {
            # Version command might not exist, that's okay
        }

        Write-ColorOutput "`nRunning '$BinName --help' to verify functionality:"
        Write-Host ""
        & $ExePath --help
        Write-Host ""
    }
    else {
        Write-ColorOutput "Installation file not found at: $ExePath" -Type "ERROR"
        exit 1
    }

    # Check if available in PATH
    $CommandAvailable = Get-Command $BinName -ErrorAction SilentlyContinue
    if ($CommandAvailable) {
        Write-ColorOutput "✅ $BinName is available in PATH"
    }
    else {
        Write-ColorOutput "$BinName is installed but not available in PATH" -Type "WARN"
        Write-ColorOutput "You may need to restart your terminal or log out and back in" -Type "WARN"
    }
}

function Main {
    Write-Host ""
    Write-Host "=========================================="
    Write-Host "     DeeSpec Installer for Windows       "
    Write-Host "=========================================="
    Write-Host ""

    # Check if running as administrator (not required, but nice to know)
    $IsAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole] "Administrator")
    if ($IsAdmin) {
        Write-ColorOutput "Running with administrator privileges"
    }
    else {
        Write-ColorOutput "Running as standard user"
    }

    # Get latest release
    $Tag = Get-LatestRelease

    # Install
    $InstalledPath = Install-DeeSpec -Tag $Tag

    # Verify
    Test-Installation -ExePath $InstalledPath

    Write-Host ""
    Write-Host "=========================================="
    Write-Host "     Installation Complete!               "
    Write-Host "=========================================="
    Write-Host ""

    Write-ColorOutput "Installation successful!"
    Write-ColorOutput "You can now use '$BinName' from any PowerShell or Command Prompt window"
    Write-Host ""
}

# Run main function
Main