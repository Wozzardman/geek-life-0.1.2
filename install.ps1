param(
    [switch]$SkipPathUpdate,
    [string]$BinDir,
    [string]$GoPath
)

$ErrorActionPreference = "Stop"

function Write-Step {
    param([string]$Message)
    Write-Host "[geek-life] $Message"
}

function Resolve-GoExecutable {
    param([string]$ExplicitPath)

    if (-not [string]::IsNullOrWhiteSpace($ExplicitPath)) {
        if (-not (Test-Path -LiteralPath $ExplicitPath -PathType Leaf)) {
            throw "The Go executable supplied with -GoPath does not exist: $ExplicitPath"
        }
        return (Resolve-Path -LiteralPath $ExplicitPath).Path
    }

    $goCommand = Get-Command go -ErrorAction SilentlyContinue
    if ($goCommand) {
        return $goCommand.Source
    }

    $candidates = @()
    if ($env:ProgramFiles) {
        $candidates += Join-Path $env:ProgramFiles "Go\bin\go.exe"
    }
    if (${env:ProgramFiles(x86)}) {
        $candidates += Join-Path ${env:ProgramFiles(x86)} "Go\bin\go.exe"
    }
    if ($env:LOCALAPPDATA) {
        $candidates += Join-Path $env:LOCALAPPDATA "Programs\Go\bin\go.exe"
    }
    if ($env:USERPROFILE) {
        $candidates += Join-Path $env:USERPROFILE "scoop\apps\go\current\bin\go.exe"
    }

    foreach ($candidate in $candidates) {
        if (Test-Path -LiteralPath $candidate -PathType Leaf) {
            return (Resolve-Path -LiteralPath $candidate).Path
        }
    }

    return $null
}

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$outputFile = Join-Path $scriptDir "geek-life.exe"
$userBinDir = if ([string]::IsNullOrWhiteSpace($BinDir)) { Join-Path $HOME "bin" } else { $BinDir }
$targetFile = Join-Path $userBinDir "geek-life.exe"
$goExecutable = Resolve-GoExecutable -ExplicitPath $GoPath

if ($goExecutable) {
    Write-Step "Building geek-life.exe with $goExecutable"
    Push-Location $scriptDir
    try {
        & $goExecutable build -o $outputFile ./app
        if ($LASTEXITCODE -ne 0) {
            throw "Go build failed with exit code $LASTEXITCODE"
        }
    } finally {
        Pop-Location
    }
} elseif (Test-Path -LiteralPath $outputFile -PathType Leaf) {
    Write-Step "Go was not found; installing the existing prebuilt geek-life.exe"
} else {
    throw "Go was not found in PATH or a common install location, and no prebuilt geek-life.exe exists. Install Go from https://go.dev/dl/, or rerun with -GoPath C:\path\to\go.exe."
}

Write-Step "Installing to $targetFile"
New-Item -ItemType Directory -Path $userBinDir -Force | Out-Null
$resolvedOutput = (Resolve-Path -LiteralPath $outputFile).Path
$resolvedTarget = [IO.Path]::GetFullPath($targetFile)
if (-not $resolvedOutput.Equals($resolvedTarget, [StringComparison]::OrdinalIgnoreCase)) {
    try {
        Copy-Item -LiteralPath $resolvedOutput -Destination $resolvedTarget -Force
    } catch [System.IO.IOException] {
        throw "Could not replace $resolvedTarget because Geek-life is still running. Exit every Geek-life instance, then rerun .\install.ps1."
    }
}

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
