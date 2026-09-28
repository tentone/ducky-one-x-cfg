$ErrorActionPreference = "Stop"

New-Item -ItemType Directory -Force -Path "dist" | Out-Null
$env:CGO_ENABLED = "1"

if (Get-Command zig -ErrorAction SilentlyContinue) {
    $env:CC = "zig cc"
} elseif (-not (Get-Command gcc -ErrorAction SilentlyContinue)) {
    throw "A C compiler is required. Install Zig or MSYS2/MinGW-w64."
}

go build -trimpath -ldflags="-s -w" -o "dist/ducky-config.exe" ./cmd/ducky-config
Write-Host "Built dist/ducky-config.exe"
