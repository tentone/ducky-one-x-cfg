package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
)

// installLightingKeyInput mirrors the web configurator's keydown listener. It
// observes keys while the application window has keyboard focus and forwards
// them to the lighting preview using the keyboard's matrix indexes.
func (u *UI) installLightingKeyInput() {
	handle := func(event *fyne.KeyEvent) {
		if event == nil || u.lighting.animator == nil {
			return
		}
		if index, ok := previewMatrixIndex(event.Name); ok {
			u.lighting.animator.Press(index)
		}
	}
	canvas := u.window.Canvas()
	if desktopCanvas, ok := canvas.(desktop.Canvas); ok {
		desktopCanvas.SetOnKeyDown(handle)
		return
	}
	canvas.SetOnTypedKey(handle)
}

func previewMatrixIndex(name fyne.KeyName) (int, bool) {
	index, ok := previewKeyIndexes[name]
	return index, ok
}

var previewKeyIndexes = map[fyne.KeyName]int{
	fyne.KeyEscape: 0,
	fyne.KeyF1:     2, fyne.KeyF2: 3, fyne.KeyF3: 4, fyne.KeyF4: 5,
	fyne.KeyF5: 6, fyne.KeyF6: 7, fyne.KeyF7: 8, fyne.KeyF8: 9,
	fyne.KeyF9: 10, fyne.KeyF10: 11, fyne.KeyF11: 12, fyne.KeyF12: 13,
	desktop.KeyPrintScreen: 14,

	fyne.KeyBackTick: 21,
	fyne.Key1:        22, fyne.Key2: 23, fyne.Key3: 24, fyne.Key4: 25,
	fyne.Key5: 26, fyne.Key6: 27, fyne.Key7: 28, fyne.Key8: 29,
	fyne.Key9: 30, fyne.Key0: 31,
	fyne.KeyMinus: 32, fyne.KeyEqual: 33, fyne.KeyBackspace: 34,
	fyne.KeyInsert: 35, fyne.KeyHome: 36, fyne.KeyPageUp: 37,
	fyne.KeyAsterisk: 40, fyne.KeyPlus: 62,

	fyne.KeyTab: 42,
	fyne.KeyQ:   43, fyne.KeyW: 44, fyne.KeyE: 45, fyne.KeyR: 46,
	fyne.KeyT: 47, fyne.KeyY: 48, fyne.KeyU: 49, fyne.KeyI: 50,
	fyne.KeyO: 51, fyne.KeyP: 52,
	fyne.KeyLeftBracket: 53, fyne.KeyRightBracket: 54, fyne.KeyBackslash: 55,
	fyne.KeyDelete: 56, fyne.KeyEnd: 57, fyne.KeyPageDown: 58,

	desktop.KeyCapsLock: 63,
	fyne.KeyA:           64, fyne.KeyS: 65, fyne.KeyD: 66, fyne.KeyF: 67,
	fyne.KeyG: 68, fyne.KeyH: 69, fyne.KeyJ: 70, fyne.KeyK: 71,
	fyne.KeyL: 72, fyne.KeySemicolon: 73, fyne.KeyApostrophe: 74,
	fyne.KeyReturn: 76,

	desktop.KeyShiftLeft: 84,
	fyne.KeyZ:            86, fyne.KeyX: 87, fyne.KeyC: 88, fyne.KeyV: 89,
	fyne.KeyB: 90, fyne.KeyN: 91, fyne.KeyM: 92,
	fyne.KeyComma: 93, fyne.KeyPeriod: 94, fyne.KeySlash: 95,
	desktop.KeyShiftRight: 96,
	fyne.KeyUp:            99,
	fyne.KeyEnter:         104,

	desktop.KeyControlLeft:  105,
	desktop.KeySuperLeft:    106,
	desktop.KeyAltLeft:      107,
	fyne.KeySpace:           110,
	desktop.KeyAltRight:     114,
	desktop.KeySuperRight:   115,
	desktop.KeyControlRight: 117,
	fyne.KeyLeft:            119,
	fyne.KeyDown:            120,
	fyne.KeyRight:           121,
}
