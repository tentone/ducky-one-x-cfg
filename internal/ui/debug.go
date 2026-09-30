package ui

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/tentone/ducky-drv/internal/protocol"
)

type debugControls struct {
	root       fyne.CanvasObject
	keyboard   *KeyboardView
	selected   *widget.Label
	digital    *widget.Label
	analog     *widget.Label
	lastEvent  *widget.Label
	selectedID int
	pressed    map[int]bool
}

func (u *UI) buildDebug() debugControls {
	t := u.i18n.T
	controls := debugControls{
		selectedID: -1,
		pressed:    make(map[int]bool),
	}
	controls.selected = widget.NewLabel(t("debug.no_selection"))
	controls.digital = widget.NewLabel(t("debug.released"))
	controls.analog = widget.NewLabel(t("debug.unavailable"))
	controls.lastEvent = widget.NewLabel(t("debug.waiting"))

	controls.keyboard = NewKeyboardView(func(index int) {
		controls.selectedID = index
		u.updateDebugDetails(&controls)
	})
	controls.keyboard.SetTapFlash(false)
	controls.keyboard.ForEach(func(index int, key *keyboardKey) {
		key.SetOverlay(t("debug.up_short"))
	})

	description := widget.NewLabel(t("debug.description"))
	description.Wrapping = fyne.TextWrapWord
	limitation := widget.NewLabel(t("debug.limitation"))
	limitation.Wrapping = fyne.TextWrapWord
	limitation.Importance = widget.WarningImportance

	selection := container.NewGridWithColumns(3,
		container.NewVBox(widget.NewLabel(t("debug.selected")), controls.selected),
		container.NewVBox(widget.NewLabel(t("debug.digital_state")), controls.digital),
		container.NewVBox(widget.NewLabel(t("debug.analog_value")), controls.analog),
	)
	controls.root = container.NewBorder(
		container.NewVBox(description, limitation, widget.NewSeparator()),
		container.NewVBox(widget.NewSeparator(), selection, controls.lastEvent),
		nil, nil,
		controls.keyboard.CanvasObject(),
	)
	return controls
}

func (u *UI) debugKeyDown(index int) {
	if u.debug.keyboard == nil {
		return
	}
	u.debug.pressed[index] = true
	u.debug.keyboard.SetOverlay(index, u.i18n.T("debug.down_short"))
	u.debug.keyboard.ForEach(func(keyIndex int, key *keyboardKey) {
		if keyIndex == index {
			key.SetDepth(1, color.NRGBA{R: 35, G: 184, B: 116, A: 210})
		}
	})
	u.debug.lastEvent.SetText(fmt.Sprintf(
		u.i18n.T("debug.last_event"), protocol.MatrixKeyLabel(index), u.i18n.T("debug.pressed"),
	))
	u.updateDebugDetails(&u.debug)
}

func (u *UI) debugKeyUp(index int) {
	if u.debug.keyboard == nil {
		return
	}
	delete(u.debug.pressed, index)
	u.debug.keyboard.SetOverlay(index, u.i18n.T("debug.up_short"))
	u.debug.keyboard.ForEach(func(keyIndex int, key *keyboardKey) {
		if keyIndex == index {
			key.SetDepth(0, color.Transparent)
		}
	})
	u.debug.lastEvent.SetText(fmt.Sprintf(
		u.i18n.T("debug.last_event"), protocol.MatrixKeyLabel(index), u.i18n.T("debug.released"),
	))
	u.updateDebugDetails(&u.debug)
}

func (u *UI) updateDebugDetails(controls *debugControls) {
	if controls == nil || controls.selectedID < 0 {
		return
	}
	controls.selected.SetText(fmt.Sprintf("%s · #%d", protocol.MatrixKeyLabel(controls.selectedID), controls.selectedID))
	if controls.pressed[controls.selectedID] {
		controls.digital.SetText(u.i18n.T("debug.pressed"))
	} else {
		controls.digital.SetText(u.i18n.T("debug.released"))
	}
	controls.analog.SetText(u.i18n.T("debug.unavailable"))
}

func (u *UI) clearDebugState() {
	if u.debug.keyboard == nil {
		return
	}
	u.debug.pressed = make(map[int]bool)
	u.debug.keyboard.ForEach(func(_ int, key *keyboardKey) {
		key.SetOverlay(u.i18n.T("debug.up_short"))
		key.SetDepth(0, color.Transparent)
		key.SetColor(theme.InputBackgroundColor())
	})
	u.debug.digital.SetText(u.i18n.T("debug.released"))
	u.debug.analog.SetText(u.i18n.T("debug.unavailable"))
	u.debug.lastEvent.SetText(u.i18n.T("debug.waiting"))
}

func (u *UI) refreshDebugTheme() {
	if u.debug.keyboard == nil {
		return
	}
	u.debug.keyboard.ForEach(func(index int, key *keyboardKey) {
		key.SetColor(theme.InputBackgroundColor())
		if u.debug.pressed[index] {
			key.SetDepth(1, color.NRGBA{R: 35, G: 184, B: 116, A: 210})
		}
	})
}
