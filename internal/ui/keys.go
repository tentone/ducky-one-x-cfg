package ui

import (
	"context"
	"fmt"
	"image/color"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/joseferrao/ducky-drv/internal/protocol"
)

type keyControls struct {
	root     fyne.CanvasObject
	layer    *widget.Select
	key      *widget.Select
	current  *widget.Label
	keyboard *KeyboardView
	mapping  []protocol.Assignment
	updating bool
}

func (u *UI) buildKeys() keyControls {
	t := u.i18n.T
	controls := keyControls{}
	controls.layer = widget.NewSelect([]string{t("layer.base"), t("layer.fn")}, nil)
	controls.layer.SetSelected(t("layer.base"))
	controls.key = widget.NewSelect(protocol.MatrixKeyOptions(), nil)
	controls.key.SetSelected(protocol.MatrixKeyOptions()[0])
	controls.current = widget.NewLabel("—")
	controls.current.TextStyle = fyne.TextStyle{Bold: true}
	controls.keyboard = NewKeyboardView(func(index int) {
		controls.key.SetSelected(matrixOption(index))
		u.showCurrentAssignment(&controls)
		u.openKeyEditor(&controls, index)
	})

	controls.key.OnChanged = func(value string) {
		index, ok := parseMatrixIndex(value)
		if ok {
			controls.keyboard.Select(index)
		}
		u.showCurrentAssignment(&controls)
	}
	controls.layer.OnChanged = func(string) {
		if controls.updating {
			return
		}
		controls.mapping = nil
		u.updateKeyKeyboard(&controls)
		u.loadKeyLayer(&controls)
	}

	load := widget.NewButtonWithIcon(t("action.load"), theme.DownloadIcon(), func() {
		u.loadKeyLayer(&controls)
	})
	reset := widget.NewButtonWithIcon(t("action.reset"), theme.DeleteIcon(), func() {
		layer := keyLayer(controls.layer.Selected, t)
		u.confirmReset(t("tab.keys"), func() {
			u.run(func(ctx context.Context) (func(), error) {
				if err := u.client.ResetKeyMap(ctx, layer); err != nil {
					return nil, err
				}
				mapping, err := u.client.KeyMap(ctx, layer)
				if err != nil {
					return nil, err
				}
				return func() {
					u.setKeyMapping(&controls, mapping)
					u.detail.SetText(t("status.reset"))
				}, nil
			})
		})
	})
	reset.Importance = widget.DangerImportance

	description := widget.NewLabel(t("keys.description"))
	description.Wrapping = fyne.TextWrapWord
	hint := widget.NewLabel(t("keys.click_edit"))
	hint.Importance = widget.LowImportance
	toolbar := container.NewBorder(
		nil, nil,
		container.NewHBox(widget.NewLabel(t("layer")), controls.layer),
		nil,
		container.NewHBox(load, reset),
	)
	keyboardCard := widget.NewCard(t("tab.keys"), "", container.NewVBox(controls.keyboard.CanvasObject(), hint))
	selection := container.NewHBox(widget.NewLabel(t("current")), controls.current)
	controls.root = container.NewBorder(
		container.NewVBox(description, widget.NewSeparator(), toolbar),
		nil, nil, nil,
		container.NewVScroll(container.NewPadded(container.NewVBox(keyboardCard, selection))),
	)
	return controls
}

func (u *UI) loadKeyLayer(controls *keyControls) {
	if u.client == nil || u.busy {
		return
	}
	layer := keyLayer(controls.layer.Selected, u.i18n.T)
	u.run(func(ctx context.Context) (func(), error) {
		if err := u.client.SetLayer(ctx, layer); err != nil {
			return nil, err
		}
		mapping, err := u.client.KeyMap(ctx, layer)
		if err != nil {
			return nil, err
		}
		return func() {
			u.setKeyMapping(controls, mapping)
			u.detail.SetText(u.i18n.T("status.loaded"))
		}, nil
	})
}

func (u *UI) setKeyMapping(controls *keyControls, mapping []protocol.Assignment) {
	controls.mapping = mapping
	u.updateKeyKeyboard(controls)
	u.showCurrentAssignment(controls)
}

func (u *UI) openKeyEditor(controls *keyControls, index int) {
	if index < 0 || index >= len(controls.mapping) {
		dialog.ShowInformation(u.i18n.T("keys.edit"), u.i18n.T("error.no_config"), u.window)
		return
	}

	t := u.i18n.T
	assignment := controls.mapping[index]
	kinds := []string{
		t("assignment.default"), t("assignment.keyboard"), t("assignment.macro"),
		t("assignment.mouse"), t("assignment.mpt"), t("assignment.disabled"),
	}
	kind := widget.NewSelect(kinds, nil)
	value := widget.NewSelect(nil, nil)
	kind.OnChanged = func(selected string) {
		options := assignmentOptions(selected, t)
		value.SetOptions(options)
		if len(options) > 0 {
			value.SetSelected(options[0])
		}
	}
	selectedKind, selectedValue := assignmentSelection(assignment, t)
	kind.SetSelected(selectedKind)
	value.SetSelected(selectedValue)

	form := widget.NewForm(
		widget.NewFormItem(t("key"), widget.NewLabel(protocol.MatrixKeyLabel(index))),
		widget.NewFormItem(t("current"), widget.NewLabel(assignment.String())),
		widget.NewFormItem(t("assignment"), kind),
		widget.NewFormItem(t("macro.value"), value),
	)
	modal := dialog.NewCustomConfirm(
		fmt.Sprintf("%s · %s", t("keys.edit"), protocol.MatrixKeyLabel(index)),
		t("action.apply"), t("action.cancel"), form,
		func(ok bool) {
			if !ok {
				return
			}
			updated, valid := selectedAssignment(kind.Selected, value.Selected, t)
			if !valid {
				dialog.ShowInformation(t("error.invalid"), t("error.invalid"), u.window)
				return
			}
			u.saveKeyAssignment(controls, index, updated)
		}, u.window,
	)
	modal.Resize(fyne.NewSize(460, 300))
	modal.Show()
}

func (u *UI) saveKeyAssignment(controls *keyControls, index int, assignment protocol.Assignment) {
	layer := keyLayer(controls.layer.Selected, u.i18n.T)
	mapping := append([]protocol.Assignment(nil), controls.mapping...)
	u.run(func(ctx context.Context) (func(), error) {
		if len(mapping) != protocol.KeyMapEntries {
			var err error
			mapping, err = u.client.KeyMap(ctx, layer)
			if err != nil {
				return nil, err
			}
		}
		mapping[index] = assignment
		if err := u.client.SetKeyMap(ctx, layer, mapping); err != nil {
			return nil, err
		}
		return func() {
			u.setKeyMapping(controls, mapping)
			controls.keyboard.Select(index)
			u.detail.SetText(u.i18n.T("status.saved"))
		}, nil
	})
}

func assignmentOptions(kind string, t func(string) string) []string {
	switch kind {
	case t("assignment.default"):
		return []string{t("assignment.default")}
	case t("assignment.macro"):
		options := make([]string, protocol.MacroSlots)
		for i := range options {
			options[i] = fmt.Sprintf("M%d", i+1)
		}
		return options
	case t("assignment.mouse"):
		return []string{"Mouse Left", "Mouse Right", "Mouse Middle"}
	case t("assignment.mpt"):
		options := make([]string, protocol.MPTPresets)
		for i := range options {
			options[i] = fmt.Sprintf("MPT%d", i+1)
		}
		return options
	case t("assignment.disabled"):
		return []string{t("assignment.disabled")}
	default:
		return protocol.KeyOptions(false)
	}
}

func selectedAssignment(kind, value string, t func(string) string) (protocol.Assignment, bool) {
	switch kind {
	case t("assignment.default"):
		return protocol.Assignment{Kind: protocol.AssignmentDefault, Code: 0}, true
	case t("assignment.macro"):
		n, err := strconv.Atoi(strings.TrimPrefix(value, "M"))
		return protocol.Assignment{Kind: protocol.AssignmentMacro, Code: byte(n)}, err == nil && n >= 1 && n <= protocol.MacroSlots
	case t("assignment.mouse"):
		code, ok := protocol.KeyCode(value)
		return protocol.Assignment{Kind: protocol.AssignmentMouse, Code: code}, ok
	case t("assignment.mpt"):
		n, err := strconv.Atoi(strings.TrimPrefix(value, "MPT"))
		return protocol.Assignment{Kind: protocol.AssignmentMPT, Code: byte(n - 1)}, err == nil && n >= 1 && n <= protocol.MPTPresets
	case t("assignment.disabled"):
		return protocol.Assignment{Kind: protocol.AssignmentDefault, Code: 0xff}, true
	default:
		code, ok := protocol.KeyCode(value)
		return protocol.Assignment{Kind: protocol.AssignmentKeyboard, Code: code}, ok
	}
}

func assignmentSelection(assignment protocol.Assignment, t func(string) string) (string, string) {
	switch assignment.Kind {
	case protocol.AssignmentDefault:
		if assignment.Code == 0xff {
			return t("assignment.disabled"), t("assignment.disabled")
		}
		return t("assignment.default"), t("assignment.default")
	case protocol.AssignmentKeyboard:
		return t("assignment.keyboard"), protocol.KeyName(assignment.Code)
	case protocol.AssignmentMacro:
		return t("assignment.macro"), fmt.Sprintf("M%d", assignment.Code)
	case protocol.AssignmentMouse:
		return t("assignment.mouse"), protocol.KeyName(assignment.Code)
	case protocol.AssignmentMPT:
		return t("assignment.mpt"), fmt.Sprintf("MPT%d", assignment.Code+1)
	default:
		return t("assignment.default"), t("assignment.default")
	}
}

func (u *UI) showCurrentAssignment(controls *keyControls) {
	index, ok := parseMatrixIndex(controls.key.Selected)
	if !ok || index >= len(controls.mapping) {
		controls.current.SetText("—")
		return
	}
	controls.current.SetText(protocol.MatrixKeyLabel(index) + " · " + controls.mapping[index].String())
}

func (u *UI) updateKeyKeyboard(controls *keyControls) {
	controls.keyboard.ForEach(func(index int, key *keyboardKey) {
		key.SetOverlay("")
		key.SetColor(theme.InputBackgroundColor())
		if index >= len(controls.mapping) {
			return
		}
		assignment := controls.mapping[index]
		key.SetOverlay(assignmentOverlay(assignment))
		switch assignment.Kind {
		case protocol.AssignmentDefault:
			if assignment.Code == 0xff {
				key.SetColor(color.NRGBA{R: 75, G: 75, B: 79, A: 255})
			}
		case protocol.AssignmentKeyboard:
			key.SetColor(blendColor(theme.InputBackgroundColor(), theme.PrimaryColor(), 0.28))
		case protocol.AssignmentMacro:
			key.SetColor(color.NRGBA{R: 82, G: 74, B: 158, A: 255})
		case protocol.AssignmentMouse:
			key.SetColor(color.NRGBA{R: 39, G: 112, B: 121, A: 255})
		case protocol.AssignmentMPT:
			key.SetColor(color.NRGBA{R: 135, G: 65, B: 155, A: 255})
		}
	})
}

func assignmentOverlay(assignment protocol.Assignment) string {
	switch assignment.Kind {
	case protocol.AssignmentDefault:
		if assignment.Code == 0xff {
			return "Off"
		}
		return ""
	case protocol.AssignmentKeyboard:
		return protocol.KeyName(assignment.Code)
	case protocol.AssignmentMacro:
		return fmt.Sprintf("M%d", assignment.Code)
	case protocol.AssignmentMouse:
		return "Mouse"
	case protocol.AssignmentMPT:
		return fmt.Sprintf("MPT%d", assignment.Code+1)
	default:
		return "?"
	}
}

func keyLayer(selected string, t func(string) string) int {
	if selected == t("layer.fn") {
		return 1
	}
	return 0
}

func matrixOption(index int) string {
	prefix := fmt.Sprintf("%03d", index)
	for _, option := range protocol.MatrixKeyOptions() {
		if strings.HasPrefix(option, prefix) {
			return option
		}
	}
	return ""
}

func parseMatrixIndex(selected string) (int, bool) {
	if len(selected) < 3 {
		return 0, false
	}
	index, err := strconv.Atoi(selected[:3])
	return index, err == nil && index >= 0 && index < protocol.MaxMatrixKeys
}
