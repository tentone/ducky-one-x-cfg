#!/usr/bin/env sh
set -eu

if [ "$(id -u)" -ne 0 ]; then
    echo "Run this installer as root, for example: sudo ./install.sh" >&2
    exit 1
fi

bundle_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
install -d /usr/bin /usr/share/applications /etc/xdg/autostart /etc/udev/rules.d
install -m 0755 "$bundle_dir/ducky-config" /usr/bin/ducky-config
install -D -m 0644 "$bundle_dir/ducky.png" /usr/share/icons/hicolor/512x512/apps/io.ducky.one-x.configurator.png
install -m 0644 "$bundle_dir/io.ducky.one-x.configurator.desktop" /usr/share/applications/io.ducky.one-x.configurator.desktop
install -m 0644 "$bundle_dir/io.ducky.one-x.configurator-autostart.desktop" /etc/xdg/autostart/io.ducky.one-x.configurator.desktop
install -m 0644 "$bundle_dir/99-ducky-one-x.rules" /etc/udev/rules.d/99-ducky-one-x.rules

if command -v udevadm >/dev/null 2>&1; then
    udevadm control --reload-rules || true
    udevadm trigger || true
fi

echo "Ducky One X Configurator installed. It will start minimized at the next desktop login."
