package shortcuts

import "testing"

func TestNativeWindowsRegistrationConflictAndRelease(t *testing.T) {
	s, _ := Parse("Ctrl+Alt+Shift+F11")
	r, err := registerNative(s)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if r != nil {
			_ = r.Unregister()
		}
	}()
	if duplicate, err := registerNative(s); err == nil {
		_ = duplicate.Unregister()
		t.Fatal("Windows allowed duplicate shortcut")
	}
	if err := r.Unregister(); err != nil {
		t.Fatal(err)
	}
	r = nil
	replacement, err := registerNative(s)
	if err != nil {
		t.Fatalf("released shortcut could not be reused: %v", err)
	}
	if err := replacement.Unregister(); err != nil {
		t.Fatal(err)
	}
}
