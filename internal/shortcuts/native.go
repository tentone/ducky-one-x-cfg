//go:build windows || darwin

package shortcuts

import "golang.design/x/hotkey"

func registerNative(s Shortcut) (registration, error) {
	mods := []hotkey.Modifier{}
	if s.Ctrl {
		mods = append(mods, hotkey.ModCtrl)
	}
	if s.Shift {
		mods = append(mods, hotkey.ModShift)
	}
	if s.Alt {
		mods = append(mods, altModifier)
	}
	if s.Super {
		mods = append(mods, superModifier)
	}
	hk := hotkey.New(mods, nativeKeys[s.Key])
	if err := hk.Register(); err != nil {
		return nil, err
	}
	r := &nativeRegistration{hk: hk, down: make(chan struct{}, 16), up: make(chan struct{}, 16), stop: make(chan struct{})}
	down, up := hk.Keydown(), hk.Keyup()
	go func() {
		for {
			select {
			case <-r.stop:
				return
			case _, ok := <-down:
				if !ok {
					return
				}
				select {
				case r.down <- struct{}{}:
				default:
				}
			case _, ok := <-up:
				if !ok {
					return
				}
				select {
				case r.up <- struct{}{}:
				default:
				}
			}
		}
	}()
	return r, nil
}

type nativeRegistration struct {
	hk       *hotkey.Hotkey
	down, up chan struct{}
	stop     chan struct{}
}

func (r *nativeRegistration) Keydown() <-chan struct{} { return r.down }
func (r *nativeRegistration) Keyup() <-chan struct{}   { return r.up }
func (r *nativeRegistration) Unregister() error {
	if err := r.hk.Unregister(); err != nil {
		return err
	}
	close(r.stop)
	return nil
}

var nativeKeys = map[string]hotkey.Key{
	"A": hotkey.KeyA, "B": hotkey.KeyB, "C": hotkey.KeyC, "D": hotkey.KeyD, "E": hotkey.KeyE, "F": hotkey.KeyF,
	"G": hotkey.KeyG, "H": hotkey.KeyH, "I": hotkey.KeyI, "J": hotkey.KeyJ, "K": hotkey.KeyK, "L": hotkey.KeyL,
	"M": hotkey.KeyM, "N": hotkey.KeyN, "O": hotkey.KeyO, "P": hotkey.KeyP, "Q": hotkey.KeyQ, "R": hotkey.KeyR,
	"S": hotkey.KeyS, "T": hotkey.KeyT, "U": hotkey.KeyU, "V": hotkey.KeyV, "W": hotkey.KeyW, "X": hotkey.KeyX, "Y": hotkey.KeyY, "Z": hotkey.KeyZ,
	"0": hotkey.Key0, "1": hotkey.Key1, "2": hotkey.Key2, "3": hotkey.Key3, "4": hotkey.Key4,
	"5": hotkey.Key5, "6": hotkey.Key6, "7": hotkey.Key7, "8": hotkey.Key8, "9": hotkey.Key9,
	"F1": hotkey.KeyF1, "F2": hotkey.KeyF2, "F3": hotkey.KeyF3, "F4": hotkey.KeyF4, "F5": hotkey.KeyF5, "F6": hotkey.KeyF6,
	"F7": hotkey.KeyF7, "F8": hotkey.KeyF8, "F9": hotkey.KeyF9, "F10": hotkey.KeyF10, "F11": hotkey.KeyF11, "F12": hotkey.KeyF12, "Space": hotkey.KeySpace,
}
