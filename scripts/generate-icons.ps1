$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
Push-Location $repoRoot
try {
    go run ./scripts/generate-icons
    if ($LASTEXITCODE -ne 0) { throw "Icon generation failed." }
    if (-not (Get-Command windres -ErrorAction SilentlyContinue)) {
        throw "MinGW-w64 windres is required to regenerate the Windows executable icons."
    }
    Push-Location packaging/windows
    try {
        windres --target=pe-x86-64 -i icon.rc -o ../../cmd/ducky-config/icon_windows_amd64.syso
        if ($LASTEXITCODE -ne 0) { throw "Windows amd64 icon generation failed." }
        windres --target=pe-i386 -i icon.rc -o ../../cmd/ducky-config/icon_windows_386.syso
        if ($LASTEXITCODE -ne 0) { throw "Windows 386 icon generation failed." }
    }
    finally { Pop-Location }
}
finally { Pop-Location }
