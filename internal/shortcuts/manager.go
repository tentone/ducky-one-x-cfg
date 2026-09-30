package shortcuts

import (
	"fmt"
	"time"
)

type registration interface {
	Keydown() <-chan struct{}
	Keyup() <-chan struct{}
	Unregister() error
}

type binding struct {
	registration
	shortcut string
	stop     chan struct{}
}

// Manager is owned by the GUI thread. Its callback runs on a worker goroutine.
type Manager struct {
	bindings  map[string]*binding
	register  func(Shortcut) (registration, error)
	onPressed func(string)
}

func New(onPressed func(string)) *Manager {
	return &Manager{bindings: make(map[string]*binding), register: registerNative, onPressed: onPressed}
}

// Set registers a replacement before releasing the existing shortcut, so a
// failed registration leaves the previous shortcut usable.
func (m *Manager) Set(id, value string) error {
	s, err := Parse(value)
	if err != nil {
		return err
	}
	value = s.String()
	old := m.bindings[id]
	if old != nil && old.shortcut == value {
		return nil
	}
	for otherID, b := range m.bindings {
		if otherID != id && value != "" && b.shortcut == value {
			return fmt.Errorf("shortcut already assigned to another profile")
		}
	}
	var next *binding
	if value != "" {
		r, err := m.register(s)
		if err != nil {
			return err
		}
		next = &binding{registration: r, shortcut: value, stop: make(chan struct{})}
	}
	if old != nil {
		if err := old.Unregister(); err != nil {
			if next != nil {
				_ = next.Unregister()
			}
			return err
		}
		close(old.stop)
	}
	delete(m.bindings, id)
	if next != nil {
		m.bindings[id] = next
		// Capture event channels before the backend resets them on unregister.
		down, up := next.Keydown(), next.Keyup()
		go m.listen(id, next.stop, down, up)
	}
	return nil
}

func (m *Manager) listen(id string, stop <-chan struct{}, down, up <-chan struct{}) {
	pressed := false
	var last time.Time
	for {
		select {
		case <-stop:
			return
		case _, ok := <-down:
			if !ok {
				return
			}
			if !pressed && time.Since(last) > time.Second {
				select {
				case <-stop:
					return
				default:
				}
				last = time.Now()
				m.onPressed(id)
			}
			pressed = true
		case _, ok := <-up:
			if !ok {
				return
			}
			pressed = false
		}
	}
}

func (m *Manager) Close() {
	for id, b := range m.bindings {
		close(b.stop)
		_ = b.Unregister()
		delete(m.bindings, id)
	}
}
