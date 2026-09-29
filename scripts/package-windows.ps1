param(
    [string]$Version = "0.1.0"
)

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
Push-Location $repoRoot
try {
    & "$PSScriptRoot\build.ps1"

    $compilerCommand = Get-Command ISCC.exe -ErrorAction SilentlyContinue
    $compilerPath = if ($compilerCommand) { $compilerCommand.Source } else { $null }
    if (-not $compilerPath) {
        $candidates = @(
            "$env:LOCALAPPDATA\Programs\Inno Setup 6\ISCC.exe",
            "$env:ProgramFiles(x86)\Inno Setup 6\ISCC.exe",
            "$env:ProgramFiles\Inno Setup 6\ISCC.exe"
        )
        foreach ($candidate in $candidates) {
            if (Test-Path -LiteralPath $candidate) {
                $compilerPath = $candidate
                break
            }
        }
    }
    if (-not $compilerPath) {
        throw "Inno Setup 6 is required to build the Windows installer. Install it, then run this script again."
    }

    $definition = Join-Path $repoRoot "packaging\windows\ducky-config.iss"
    & $compilerPath "/DAppVersion=$Version" "/DRepoRoot=$repoRoot" $definition
    if ($LASTEXITCODE -ne 0) {
        throw "Inno Setup failed with exit code $LASTEXITCODE."
    }
    Write-Host "Built dist/Ducky-One-X-Configurator-Setup-$Version.exe"
}
finally {
    Pop-Location
}
