package startup

import (
	"bytes"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLaunchAgentPreservesExecutablePath(t *testing.T) {
	path := `/Applications/Ducky & "Friends" <test>.app/Contents/MacOS/ducky-config`
	decoder := xml.NewDecoder(bytes.NewReader(launchAgent(path)))
	var values []string
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if start, ok := token.(xml.StartElement); ok && start.Name.Local == "string" {
			var value string
			if err := decoder.DecodeElement(&value, &start); err != nil {
				t.Fatal(err)
			}
			values = append(values, value)
		}
	}
	if len(values) != 4 || values[1] != path || values[2] != "--autostart" {
		t.Fatalf("unexpected launch agent arguments: %q", values)
	}
}

func TestDesktopEntryEscapesExecMetacharacters(t *testing.T) {
	entry := string(desktopEntry(`/home/user/Ducky "$test" 100%/ducky-config`))
	want := `Exec="/home/user/Ducky \\"\\$test\\" 100%%/ducky-config" --autostart`
	if !strings.Contains(entry, want+"\n") {
		t.Fatalf("unexpected desktop command: %s", entry)
	}
}

func TestStartupFileLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "autostart", id+".desktop")
	if err := writeEntry(path, desktopEntry("/opt/Ducky/ducky-config")); err != nil {
		t.Fatal(err)
	}
	if !exists(path) {
		t.Fatal("startup entry was not created")
	}
	if err := removeEntry(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("startup entry remains: %v", err)
	}
	if err := removeEntry(path); err != nil {
		t.Fatalf("removing missing entry: %v", err)
	}
}
