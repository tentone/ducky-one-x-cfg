package shortcuts

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/xproto"
)

type x11Registration struct {
	conn      *xgb.Conn
	root      xproto.Window
	key       xproto.Keycode
	modifiers []uint16
	down, up  chan struct{}
}

func registerNative(s Shortcut) (registration, error) {
	if strings.EqualFold(os.Getenv("XDG_SESSION_TYPE"), "wayland") {
		return nil, fmt.Errorf("global profile shortcuts require an X11 session on Linux")
	}
	conn, err := xgb.NewConn()
	if err != nil {
		return nil, fmt.Errorf("connect to X11: %w", err)
	}
	setup := xproto.Setup(conn)
	mapping, err := xproto.GetKeyboardMapping(conn, setup.MinKeycode, byte(int(setup.MaxKeycode)-int(setup.MinKeycode)+1)).Reply()
	if err != nil {
		conn.Close()
		return nil, err
	}
	symbol := xproto.Keysym(0)
	switch {
	case len(s.Key) == 1:
		symbol = xproto.Keysym(strings.ToLower(s.Key)[0])
	case s.Key == "Space":
		symbol = 0x20
	case strings.HasPrefix(s.Key, "F"):
		n, _ := strconv.Atoi(s.Key[1:])
		symbol = xproto.Keysym(0xffbe + n - 1)
	}
	var key xproto.Keycode
	for index, sym := range mapping.Keysyms {
		if sym == symbol {
			key = setup.MinKeycode + xproto.Keycode(index/int(mapping.KeysymsPerKeycode))
			break
		}
	}
	if key == 0 {
		conn.Close()
		return nil, fmt.Errorf("shortcut key %q is unavailable in the current keyboard layout", s.Key)
	}
	var modifiers uint16
	if s.Ctrl {
		modifiers |= xproto.ModMaskControl
	}
	if s.Alt {
		modifiers |= xproto.ModMask1
	}
	if s.Shift {
		modifiers |= xproto.ModMaskShift
	}
	if s.Super {
		modifiers |= xproto.ModMask4
	}
	// Ignore CapsLock and the actual modifier assigned to NumLock.
	lockMask := uint16(xproto.ModMaskLock)
	modMap, err := xproto.GetModifierMapping(conn).Reply()
	if err != nil {
		conn.Close()
		return nil, err
	}
	for index, code := range modMap.Keycodes {
		if code < setup.MinKeycode || code > setup.MaxKeycode {
			continue
		}
		start := int(code-setup.MinKeycode) * int(mapping.KeysymsPerKeycode)
		for _, sym := range mapping.Keysyms[start : start+int(mapping.KeysymsPerKeycode)] {
			if sym == 0xff7f {
				lockMask |= uint16(1 << (index / int(modMap.KeycodesPerModifier)))
			}
		}
	}
	r := &x11Registration{conn: conn, root: setup.DefaultScreen(conn).Root, key: key, down: make(chan struct{}, 16), up: make(chan struct{}, 16)}
	for subset := uint16(0); subset <= lockMask; subset++ {
		if subset & ^lockMask != 0 {
			continue
		}
		mods := modifiers | subset
		if err := xproto.GrabKeyChecked(conn, false, r.root, mods, key, xproto.GrabModeAsync, xproto.GrabModeAsync).Check(); err != nil {
			conn.Close()
			return nil, fmt.Errorf("shortcut is already in use: %w", err)
		}
		r.modifiers = append(r.modifiers, mods)
	}
	go func() {
		for {
			event, err := conn.WaitForEvent()
			if err != nil || event == nil {
				return
			}
			switch event := event.(type) {
			case xproto.KeyPressEvent:
				if event.Detail == key {
					select {
					case r.down <- struct{}{}:
					default:
					}
				}
			case xproto.KeyReleaseEvent:
				if event.Detail == key {
					select {
					case r.up <- struct{}{}:
					default:
					}
				}
			}
		}
	}()
	return r, nil
}

func (r *x11Registration) Keydown() <-chan struct{} { return r.down }
func (r *x11Registration) Keyup() <-chan struct{}   { return r.up }
func (r *x11Registration) Unregister() error {
	// Closing an X11 connection releases all passive grabs owned by it.
	r.conn.Close()
	return nil
}
