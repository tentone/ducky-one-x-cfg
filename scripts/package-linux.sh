#!/usr/bin/env sh
set -eu

version=${1:-0.1.0}
repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
dist="$repo_root/dist"
work="$dist/linux-package"
bundle="$work/bundle"
debroot="$work/deb"

rm -rf "$work"
mkdir -p "$bundle" "$debroot/DEBIAN" "$debroot/usr/bin" \
    "$debroot/usr/share/applications" "$debroot/etc/xdg/autostart" "$debroot/etc/udev/rules.d"

cd "$repo_root"
CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o "$bundle/ducky-config" ./cmd/ducky-config
cp packaging/linux/io.ducky.one-x.configurator.desktop "$bundle/"
cp packaging/linux/io.ducky.one-x.configurator-autostart.desktop "$bundle/"
cp packaging/linux/install.sh packaging/linux/uninstall.sh "$bundle/"
cp packaging/99-ducky-one-x.rules "$bundle/"
chmod 0755 "$bundle/ducky-config" "$bundle/install.sh" "$bundle/uninstall.sh"

tar -C "$bundle" -czf "$dist/ducky-one-x-configurator-$version-linux.tar.gz" .
echo "Built dist/ducky-one-x-configurator-$version-linux.tar.gz"

if command -v dpkg-deb >/dev/null 2>&1; then
    arch=$(dpkg --print-architecture)
    install -m 0755 "$bundle/ducky-config" "$debroot/usr/bin/ducky-config"
    install -m 0644 "$bundle/io.ducky.one-x.configurator.desktop" "$debroot/usr/share/applications/io.ducky.one-x.configurator.desktop"
    install -m 0644 "$bundle/io.ducky.one-x.configurator-autostart.desktop" "$debroot/etc/xdg/autostart/io.ducky.one-x.configurator.desktop"
    install -m 0644 "$bundle/99-ducky-one-x.rules" "$debroot/etc/udev/rules.d/99-ducky-one-x.rules"
    cat > "$debroot/DEBIAN/control" <<EOF
Package: ducky-one-x-configurator
Version: $version
Section: utils
Priority: optional
Architecture: $arch
Maintainer: Ducky One X Configurator contributors
Depends: libc6, libgl1, libx11-6, libxcursor1, libxrandr2, libxinerama1, libxi6, libxxf86vm1, libudev1
Description: Offline Ducky One X keyboard configurator
 Native Fyne application for key mappings, lighting, actuation, MPT,
 macros, onboard profiles, and software-managed profiles.
EOF
    cat > "$debroot/DEBIAN/postinst" <<'EOF'
#!/bin/sh
set -e
if command -v udevadm >/dev/null 2>&1; then
    udevadm control --reload-rules || true
    udevadm trigger || true
fi
exit 0
EOF
    cat > "$debroot/DEBIAN/postrm" <<'EOF'
#!/bin/sh
set -e
if command -v udevadm >/dev/null 2>&1; then
    udevadm control --reload-rules || true
fi
exit 0
EOF
    chmod 0755 "$debroot/DEBIAN/postinst" "$debroot/DEBIAN/postrm"
    dpkg-deb --root-owner-group --build "$debroot" "$dist/ducky-one-x-configurator_${version}_${arch}.deb"
    echo "Built dist/ducky-one-x-configurator_${version}_${arch}.deb"
fi
