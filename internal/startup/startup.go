// Package startup manages login startup entries for the current user.
package startup

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const id = "io.ducky.one-x.configurator"

func executable() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Abs(path)
}

func writeEntry(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func removeEntry(path string) error {
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func desktopEntry(path string) []byte {
	// Desktop Exec quoting has both string and command-line escaping layers.
	path = strings.NewReplacer(`\`, `\\\\`, `"`, `\\"`, "`", "\\\\`", "$", `\\$`, "%", "%%").Replace(path)
	return []byte(fmt.Sprintf("[Desktop Entry]\nType=Application\nName=Ducky One X Configurator\nExec=\"%s\" --autostart\nIcon=%s\nTerminal=false\nX-GNOME-Autostart-enabled=true\n", path, id))
}

func launchAgent(path string) []byte {
	var escaped bytes.Buffer
	_ = xml.EscapeText(&escaped, []byte(path))
	return []byte(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>Label</key><string>%s</string>
<key>ProgramArguments</key><array><string>%s</string><string>--autostart</string></array>
<key>RunAtLoad</key><true/>
<key>LimitLoadToSessionType</key><string>Aqua</string>
</dict></plist>
`, id, escaped.String()))
}
