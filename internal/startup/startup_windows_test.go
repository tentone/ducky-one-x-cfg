package startup

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"golang.org/x/sys/windows/registry"
)

func TestUserStartupRegistryLifecycle(t *testing.T) {
	// Isolate the test from the real Windows Run key and the user's settings.
	path := fmt.Sprintf(`Software\DuckyOneXConfiguratorTests\%d`, time.Now().UnixNano())
	t.Cleanup(func() { _ = registry.DeleteKey(registry.CURRENT_USER, path) })
	if err := setUserEntry(path, true); err != nil {
		t.Fatal(err)
	}
	key, err := registry.OpenKey(registry.CURRENT_USER, path, registry.QUERY_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	defer key.Close()
	exe, err := executable()
	if err != nil {
		t.Fatal(err)
	}
	command, _, err := key.GetStringValue(valueName)
	if err != nil || command != `"`+exe+`" --autostart` {
		t.Fatalf("startup command = %q, %v", command, err)
	}
	if err := setUserEntry(path, false); err != nil {
		t.Fatal(err)
	}
	if _, _, err := key.GetStringValue(valueName); !errors.Is(err, registry.ErrNotExist) {
		t.Fatalf("startup value remains: %v", err)
	}
	if err := setUserEntry(path, false); err != nil {
		t.Fatal(err)
	}
}
