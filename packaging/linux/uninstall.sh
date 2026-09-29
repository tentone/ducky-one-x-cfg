#!/usr/bin/env sh
set -eu

if [ "$(id -u)" -ne 0 ]; then
    echo "Run this uninstaller as root, for example: sudo ./uninstall.sh" >&2
    exit 1
fi

rm -f /usr/bin/ducky-config
rm -f /usr/share/applications/io.ducky.one-x.configurator.desktop
rm -f /etc/xdg/autostart/io.ducky.one-x.configurator.desktop
rm -f /etc/udev/rules.d/99-ducky-one-x.rules

if command -v udevadm >/dev/null 2>&1; then
    udevadm control --reload-rules || true
fi

echo "Ducky One X Configurator uninstalled. User profiles and preferences were left in place."
