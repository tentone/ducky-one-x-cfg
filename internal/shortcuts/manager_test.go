package shortcuts

import (
	"errors"
	"testing"
	"time"
)

type fakeRegistration struct {
	down, up chan struct{}
	released bool
}

func (r *fakeRegistration) Keydown() <-chan struct{} { return r.down }
func (r *fakeRegistration) Keyup() <-chan struct{}   { return r.up }
func (r *fakeRegistration) Unregister() error        { r.released = true; return nil }

func TestReplacementFailurePreservesShortcutAndHoldingDoesNotRepeat(t *testing.T) {
	triggered := make(chan string, 8)
	m := New(func(id string) { triggered <- id })
	defer m.Close()
	var original *fakeRegistration
	m.register = func(s Shortcut) (registration, error) {
		if s.Key == "2" {
			return nil, errors.New("claimed by another app")
		}
		original = &fakeRegistration{down: make(chan struct{}), up: make(chan struct{})}
		return original, nil
	}
	if err := m.Set("profile-a", "Ctrl+Alt+1"); err != nil {
		t.Fatal(err)
	}
	if err := m.Set("profile-a", "Ctrl+Alt+2"); err == nil {
		t.Fatal("conflicting replacement accepted")
	}
	if original.released {
		t.Fatal("failed replacement released the existing shortcut")
	}
	if err := m.Set("profile-b", "Alt+Ctrl+1"); err == nil {
		t.Fatal("duplicate profile shortcut accepted")
	}
	original.down <- struct{}{}
	select {
	case id := <-triggered:
		if id != "profile-a" {
			t.Fatal(id)
		}
	case <-time.After(time.Second):
		t.Fatal("shortcut did not trigger")
	}
	original.down <- struct{}{}
	select {
	case <-triggered:
		t.Fatal("holding a shortcut repeated the load")
	case <-time.After(30 * time.Millisecond):
	}
	if err := m.Set("profile-a", ""); err != nil {
		t.Fatal(err)
	}
	if !original.released {
		t.Fatal("clearing did not release shortcut")
	}
}
