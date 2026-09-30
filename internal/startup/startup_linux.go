package startup

import (
	"os"
	"path/filepath"
)

func entryPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "autostart", id+".desktop"), nil
}

func LegacyEnabled() bool { return exists("/etc/xdg/autostart/" + id + "-autostart.desktop") }
func Enabled() bool {
	path, err := entryPath()
	return err == nil && (exists(path) || LegacyEnabled())
}

func SetEnabled(enabled bool) error {
	path, err := entryPath()
	if err != nil {
		return err
	}
	if !enabled {
		return removeEntry(path)
	}
	if LegacyEnabled() {
		return removeEntry(path)
	}
	exe, err := executable()
	if err != nil {
		return err
	}
	return writeEntry(path, desktopEntry(exe))
}
