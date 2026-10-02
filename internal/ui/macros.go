package ui

import (
	"context"
	"fmt"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/tentone/ducky-drv/internal/protocol"
)

type macroControls struct {
	root     fyne.CanvasObject
	slot     *widget.Select
	kind     *widget.Select
	key      *widget.Select
	value    *widget.Entry
	update   *widget.Button
	list     *macroActionList
	actions  []protocol.MacroAction
	selected widget.ListItemID
}

func (u *UI) buildMacros() *macroControls {
	t := u.i18n.T
	controls := &macroControls{selected: -1}
	slots := make([]string, protocol.MacroSlots)
	for i := range slots {
		slots[i] = fmt.Sprintf("M%d", i+1)
	}
	controls.slot = widget.NewSelect(slots, nil)
	controls.slot.SetSelected(slots[0])
	kinds := []string{t("macro.click"), t("macro.press"), t("macro.release"), t("macro.delay"), t("macro.text")}
	controls.kind = widget.NewSelect(kinds, nil)
	controls.kind.SetSelected(kinds[0])
	controls.key = widget.NewSelect(protocol.KeyOptions(true), nil)
	controls.key.SetSelected("A")
	controls.value = widget.NewEntry()
	controls.value.SetPlaceHolder(t("macro.placeholder"))
	controls.kind.OnChanged = func(value string) {
		if value == t("macro.delay") || value == t("macro.text") {
			controls.key.Disable()
			controls.value.Enable()
		} else {
			controls.key.Enable()
			controls.value.Disable()
		}
	}
	controls.value.Disable()

	var autoApply func()
	var update, remove *widget.Button
	selectAction := func(id int) {
		controls.selected = id
		if id < 0 || id >= len(controls.actions) {
			update.Disable()
			remove.Disable()
			return
		}
		action := controls.actions[id]
		controls.kind.SetSelected(t(macroKindID(action.Kind)))
		switch action.Kind {
		case protocol.MacroDelay:
			controls.value.SetText(strconv.Itoa(action.DelayMS))
		case protocol.MacroText:
			controls.value.SetText(action.Text)
		default:
			if len(action.Keys) > 0 {
				controls.key.SetSelected(protocol.KeyName(action.Keys[0]))
			}
		}
		update.Enable()
		remove.Enable()
	}
	controls.list = newMacroActionList(controls, t, selectAction, func() {
		if autoApply != nil {
			autoApply()
		}
	})
	add := widget.NewButtonWithIcon(t("action.add"), theme.ContentAddIcon(), func() {
		action, ok := macroActionFromInputs(controls, t)
		if !ok {
			dialog.ShowInformation(t("error.invalid"), t("error.invalid"), u.window)
			return
		}
		controls.actions = append(controls.actions, action)
		selectAction(len(controls.actions) - 1)
		controls.list.Refresh()
		controls.list.ScrollToBottom()
		if autoApply != nil {
			autoApply()
		}
	})
	update = widget.NewButtonWithIcon(t("macro.update"), theme.ConfirmIcon(), func() {
		id := controls.selected
		if id < 0 || id >= len(controls.actions) {
			return
		}
		action, ok := macroActionFromInputs(controls, t)
		if !ok {
			dialog.ShowInformation(t("error.invalid"), t("error.invalid"), u.window)
			return
		}
		previous := controls.actions[id]
		// Keep firmware fields that this editor does not change.
		if action.Kind == protocol.MacroDelay && previous.Kind == protocol.MacroDelay {
			action.RandomMS = previous.RandomMS
		}
		if len(action.Keys) > 0 && len(previous.Keys) > 0 && action.Keys[0] == previous.Keys[0] {
			action.Keys = append([]byte(nil), previous.Keys...)
		}
		controls.actions[id] = action
		controls.list.Refresh()
		if autoApply != nil {
			autoApply()
		}
	})
	controls.update = update
	remove = widget.NewButtonWithIcon(t("action.remove"), theme.ContentRemoveIcon(), func() {
		if controls.selected < 0 || controls.selected >= len(controls.actions) {
			return
		}
		id := int(controls.selected)
		controls.actions = append(controls.actions[:id], controls.actions[id+1:]...)
		controls.selected = -1
		controls.list.UnselectAll()
		controls.list.Refresh()
		if autoApply != nil {
			autoApply()
		}
	})
	selectAction(-1)

	load := widget.NewButtonWithIcon(t("action.load"), theme.DownloadIcon(), func() {
		slot := macroSlot(controls.slot.Selected)
		u.run(func(ctx context.Context) (func(), error) {
			actions, err := u.client.Macro(ctx, slot)
			if err != nil {
				return nil, err
			}
			return func() {
				controls.actions = actions
				controls.selected = -1
				controls.list.UnselectAll()
				controls.list.Refresh()
				u.detail.SetText(t("status.loaded"))
			}, nil
		})
	})
	writeMacro := func(slot int, actions []protocol.MacroAction) {
		u.run(func(ctx context.Context) (func(), error) {
			if err := u.client.SetMacro(ctx, slot, actions); err != nil {
				return nil, err
			}
			return func() { u.detail.SetText(t("status.saved")) }, nil
		})
	}
	applyMacro := func() {
		slot := macroSlot(controls.slot.Selected)
		actions := append([]protocol.MacroAction(nil), controls.actions...)
		writeMacro(slot, actions)
	}
	save := widget.NewButtonWithIcon(t("action.apply"), theme.ConfirmIcon(), applyMacro)
	autoApply = func() {
		slot := macroSlot(controls.slot.Selected)
		actions := append([]protocol.MacroAction(nil), controls.actions...)
		u.scheduleAutoSync(fmt.Sprintf("macro:%d", slot), func() { writeMacro(slot, actions) })
	}
	clear := widget.NewButtonWithIcon(t("action.reset"), theme.DeleteIcon(), func() {
		slot := macroSlot(controls.slot.Selected)
		u.confirmReset(controls.slot.Selected, func() {
			u.run(func(ctx context.Context) (func(), error) {
				if err := u.client.SetMacro(ctx, slot, nil); err != nil {
					return nil, err
				}
				return func() {
					controls.actions = nil
					controls.list.UnselectAll()
					controls.list.Refresh()
					u.detail.SetText(t("status.reset"))
				}, nil
			})
		})
	})
	clear.Importance = widget.DangerImportance

	editor := widget.NewCard(t("macro.edit"), "", widget.NewForm(
		widget.NewFormItem(t("macro.kind"), controls.kind),
		widget.NewFormItem(t("output.key"), controls.key),
		widget.NewFormItem(t("macro.value"), controls.value),
		widget.NewFormItem("", container.NewHBox(update, remove)),
	))
	paletteHint := widget.NewLabel(t("macro.palette"))
	paletteHint.Wrapping = fyne.TextWrapWord
	palette := container.NewVBox(paletteHint)
	for _, kind := range []protocol.MacroActionKind{protocol.MacroClick, protocol.MacroPress, protocol.MacroRelease, protocol.MacroDelay, protocol.MacroText} {
		kind := kind
		palette.Add(newMacroDragItem(t(macroKindID(kind)), func() {
			controls.list.insert(kind, len(controls.actions))
			controls.list.ScrollToBottom()
		}, func(pos fyne.Position, end bool) {
			controls.list.drag(kind, -1, pos, end)
		}))
	}
	description := widget.NewLabel(t("macro.description"))
	description.Wrapping = fyne.TextWrapWord
	toolbar := container.NewBorder(nil, nil, container.NewHBox(widget.NewLabel(t("macro.slot")), controls.slot), nil, container.NewHBox(load, save, clear))
	editorScroll := container.NewVScroll(container.NewPadded(container.NewVBox(palette, widget.NewSeparator(), editor, add)))
	workspace := container.NewHSplit(container.NewPadded(controls.list), editorScroll)
	workspace.Offset = 0.62
	controls.root = container.NewBorder(
		container.NewVBox(description, widget.NewSeparator(), toolbar), nil, nil, nil,
		workspace,
	)
	return controls
}

func macroActionFromInputs(controls *macroControls, t func(string) string) (protocol.MacroAction, bool) {
	switch controls.kind.Selected {
	case t("macro.delay"):
		milliseconds, err := strconv.Atoi(controls.value.Text)
		if err != nil || milliseconds < 1 || milliseconds > 999 {
			return protocol.MacroAction{}, false
		}
		return protocol.MacroAction{Kind: protocol.MacroDelay, DelayMS: milliseconds}, true
	case t("macro.text"):
		if controls.value.Text == "" {
			return protocol.MacroAction{}, false
		}
		return protocol.MacroAction{Kind: protocol.MacroText, Text: controls.value.Text}, true
	default:
		code, ok := protocol.KeyCode(controls.key.Selected)
		if !ok {
			return protocol.MacroAction{}, false
		}
		kind := protocol.MacroClick
		if controls.kind.Selected == t("macro.press") {
			kind = protocol.MacroPress
		} else if controls.kind.Selected == t("macro.release") {
			kind = protocol.MacroRelease
		}
		return protocol.MacroAction{Kind: kind, Keys: []byte{code}}, true
	}
}

func macroSlot(value string) int {
	slot, err := strconv.Atoi(value[1:])
	if err != nil || slot < 1 || slot > protocol.MacroSlots {
		return 1
	}
	return slot
}
