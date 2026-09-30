#!/usr/bin/env sh
set -eu

if [ "$(uname -s)" != "Darwin" ]; then
    echo "The macOS installer must be built on macOS." >&2
    exit 1
fi

version=${1:-0.1.0}
repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
dist="$repo_root/dist"
work="$dist/macos-package"
root="$work/root"
app="$root/Applications/Ducky One X Configurator.app"

rm -rf "$work"
mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources" "$root/Library/LaunchAgents"

cd "$repo_root"
CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o "$app/Contents/MacOS/ducky-config" ./cmd/ducky-config
sed "s/__VERSION__/$version/g" packaging/macos/Info.plist > "$app/Contents/Info.plist"
cp assets/ducky.icns "$app/Contents/Resources/ducky.icns"
cp packaging/macos/io.ducky.one-x.configurator.plist "$root/Library/LaunchAgents/"
chmod 0755 "$app/Contents/MacOS/ducky-config"
chmod 0644 "$app/Contents/Info.plist" "$root/Library/LaunchAgents/io.ducky.one-x.configurator.plist"

if [ -n "${MACOS_SIGN_IDENTITY:-}" ]; then
    codesign --force --deep --options runtime --sign "$MACOS_SIGN_IDENTITY" "$app"
fi

output="$dist/Ducky-One-X-Configurator-$version.pkg"
if [ -n "${MACOS_INSTALLER_IDENTITY:-}" ]; then
    pkgbuild --root "$root" --ownership recommended --identifier io.ducky.one-x.configurator --version "$version" \
        --install-location / --sign "$MACOS_INSTALLER_IDENTITY" "$output"
else
    pkgbuild --root "$root" --ownership recommended --identifier io.ducky.one-x.configurator --version "$version" \
        --install-location / "$output"
fi
echo "Built $output"
