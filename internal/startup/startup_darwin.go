package startup

import (
	"os"
	"path/filepath"
)

func entryPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", id+".plist"), nil
}

func LegacyEnabled() bool { return exists("/Library/LaunchAgents/" + id + ".plist") }
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
	return writeEntry(path, launchAgent(exe))
}
