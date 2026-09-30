package startup

import (
	"errors"
	"golang.org/x/sys/windows/registry"
)

const runKey = `Software\Microsoft\Windows\CurrentVersion\Run`
const valueName = "DuckyOneXConfigurator"

func registered(root registry.Key) bool {
	key, err := registry.OpenKey(root, runKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer key.Close()
	value, _, err := key.GetStringValue(valueName)
	return err == nil && value != ""
}

// LegacyEnabled reports the system-wide startup entry supplied by installers.
func LegacyEnabled() bool { return registered(registry.LOCAL_MACHINE) }
func Enabled() bool       { return registered(registry.CURRENT_USER) || LegacyEnabled() }

func SetEnabled(enabled bool) error {
	// Keep the installer entry as the single launcher when it is present.
	if enabled && LegacyEnabled() {
		return setUserEntry(runKey, false)
	}
	return setUserEntry(runKey, enabled)
}

func setUserEntry(keyPath string, enabled bool) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, keyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	if !enabled {
		err = key.DeleteValue(valueName)
		if errors.Is(err, registry.ErrNotExist) {
			return nil
		}
		return err
	}
	path, err := executable()
	if err != nil {
		return err
	}
	return key.SetStringValue(valueName, `"`+path+`" --autostart`)
}
