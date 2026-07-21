param(
    [switch]$SkipPathUpdate
)

$ErrorActionPreference = "Stop"

function Write-Step {
    param([string]$Message)
    Write-Host "[geek-life] $Message"
}

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$outputFile = Join-Path $scriptDir "geek-life.exe"
$userBinDir = Join-Path $HOME "bin"
$targetFile = Join-Path $userBinDir "geek-life.exe"

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw "Go is not installed or not in PATH. Install Go from https://go.dev/dl/ and rerun this script."
}

Write-Step "Building geek-life.exe"
Push-Location $scriptDir
try {
    go build -o $outputFile ./app
} finally {
    Pop-Location
}

Write-Step "Installing to $targetFile"
New-Item -ItemType Directory -Path $userBinDir -Force | Out-Null
Copy-Item -Path $outputFile -Destination $targetFile -Force

if (-not $SkipPathUpdate) {
    $currentUserPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $alreadyInPath = $false

    if ($currentUserPath) {
        $pathEntries = $currentUserPath -split ";"
        foreach ($entry in $pathEntries) {
            if ($entry.Trim().TrimEnd('\\') -ieq $userBinDir.Trim().TrimEnd('\\')) {
                $alreadyInPath = $true
                break
            }
        }
    }

    if (-not $alreadyInPath) {
        $newUserPath = if ([string]::IsNullOrWhiteSpace($currentUserPath)) {
            $userBinDir
        } else {
            "$currentUserPath;$userBinDir"
        }

        [Environment]::SetEnvironmentVariable("Path", $newUserPath, "User")

        if (-not ($env:Path -split ";" | Where-Object { $_.Trim().TrimEnd('\\') -ieq $userBinDir.Trim().TrimEnd('\\') })) {
            $env:Path = "$env:Path;$userBinDir"
        }

        Write-Step "Added $userBinDir to your User PATH"
    } else {
        Write-Step "$userBinDir is already in your User PATH"
    }
}

Write-Step "Install complete. Run: geek-life"