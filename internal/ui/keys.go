package ui

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/joseferrao/ducky-drv/internal/protocol"
)

type keyControls struct {
	root     fyne.CanvasObject
	layer    *widget.Select
	key      *widget.Select
	kind     *widget.Select
	value    *widget.Select
	current  *widget.Label
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

	kinds := []string{
		t("assignment.default"), t("assignment.keyboard"), t("assignment.macro"), t("assignment.mouse"),
		t("assignment.mpt"), t("assignment.disabled"),
	}
	controls.kind = widget.NewSelect(kinds, nil)
	controls.kind.SetSelected(kinds[0])
	controls.value = widget.NewSelect(protocol.KeyOptions(false), nil)
	controls.value.SetSelected("A")

	controls.kind.OnChanged = func(string) { u.refreshKeyAssignmentOptions(&controls) }
	controls.key.OnChanged = func(string) { u.showCurrentAssignment(&controls) }
	controls.layer.OnChanged = func(string) {
		controls.mapping = nil
		controls.current.SetText("—")
	}

	load := widget.NewButtonWithIcon(t("action.load"), theme.DownloadIcon(), func() {
		layer := keyLayer(controls.layer.Selected, t)
		u.run(func(ctx context.Context) (func(), error) {
			if err := u.client.SetLayer(ctx, layer); err != nil {
				return nil, err
			}
			mapping, err := u.client.KeyMap(ctx, layer)
			if err != nil {
				return nil, err
			}
			return func() {
				controls.mapping = mapping
				u.showCurrentAssignment(&controls)
				u.detail.SetText(t("status.loaded"))
			}, nil
		})
	})
	apply := widget.NewButtonWithIcon(t("action.apply"), theme.ConfirmIcon(), func() {
		layer := keyLayer(controls.layer.Selected, t)
		index, ok := parseMatrixIndex(controls.key.Selected)
		if !ok {
			return
		}
		assignment, ok := u.selectedAssignment(&controls)
		if !ok {
			return
		}
		u.run(func(ctx context.Context) (func(), error) {
			mapping := append([]protocol.Assignment(nil), controls.mapping...)
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
				controls.mapping = mapping
				u.showCurrentAssignment(&controls)
				u.detail.SetText(t("status.saved"))
			}, nil
		})
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
					controls.mapping = mapping
					u.showCurrentAssignment(&controls)
					u.detail.SetText(t("status.reset"))
				}, nil
			})
		})
	})
	reset.Importance = widget.DangerImportance

	form := widget.NewForm(
		widget.NewFormItem(t("layer"), controls.layer),
		widget.NewFormItem(t("key"), controls.key),
		widget.NewFormItem(t("current"), controls.current),
		widget.NewFormItem(t("assignment"), controls.kind),
		widget.NewFormItem(t("macro.value"), controls.value),
	)
	description := widget.NewLabel(t("keys.description"))
	description.Wrapping = fyne.TextWrapWord
	controls.root = container.NewBorder(
		container.NewVBox(description, widget.NewSeparator()),
		container.NewHBox(load, apply, reset), nil, nil,
		container.NewPadded(container.NewVBox(widget.NewCard(t("tab.keys"), "", form))),
	)
	return controls
}

func (u *UI) refreshKeyAssignmentOptions(controls *keyControls) {
	t := u.i18n.T
	var options []string
	switch controls.kind.Selected {
	case t("assignment.default"):
		options = []string{t("assignment.default")}
	case t("assignment.macro"):
		for i := 1; i <= protocol.MacroSlots; i++ {
			options = append(options, fmt.Sprintf("M%d", i))
		}
	case t("assignment.mouse"):
		options = []string{"Mouse Left", "Mouse Right", "Mouse Middle"}
	case t("assignment.mpt"):
		for i := 1; i <= protocol.MPTPresets; i++ {
			options = append(options, fmt.Sprintf("MPT%d", i))
		}
	case t("assignment.disabled"):
		options = []string{t("assignment.disabled")}
	default:
		options = protocol.KeyOptions(false)
	}
	controls.value.SetOptions(options)
	if len(options) > 0 && !controls.updating {
		controls.value.SetSelected(options[0])
	}
}

func (u *UI) selectedAssignment(controls *keyControls) (protocol.Assignment, bool) {
	t := u.i18n.T
	value := controls.value.Selected
	switch controls.kind.Selected {
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

func (u *UI) showCurrentAssignment(controls *keyControls) {
	index, ok := parseMatrixIndex(controls.key.Selected)
	if !ok || index >= len(controls.mapping) {
		controls.current.SetText("—")
		return
	}
	assignment := controls.mapping[index]
	controls.current.SetText(assignment.String())
	controls.updating = true
	defer func() { controls.updating = false }()
	t := u.i18n.T
	switch assignment.Kind {
	case protocol.AssignmentDefault:
		if assignment.Code == 0xff {
			controls.kind.SetSelected(t("assignment.disabled"))
			controls.value.SetSelected(t("assignment.disabled"))
		} else {
			controls.kind.SetSelected(t("assignment.default"))
			controls.value.SetSelected(t("assignment.default"))
		}
	case protocol.AssignmentKeyboard:
		controls.kind.SetSelected(t("assignment.keyboard"))
		controls.value.SetSelected(protocol.KeyName(assignment.Code))
	case protocol.AssignmentMacro:
		controls.kind.SetSelected(t("assignment.macro"))
		controls.value.SetSelected(fmt.Sprintf("M%d", assignment.Code))
	case protocol.AssignmentMouse:
		controls.kind.SetSelected(t("assignment.mouse"))
		controls.value.SetSelected(protocol.KeyName(assignment.Code))
	case protocol.AssignmentMPT:
		controls.kind.SetSelected(t("assignment.mpt"))
		controls.value.SetSelected(fmt.Sprintf("MPT%d", assignment.Code+1))
	}
}

func keyLayer(selected string, t func(string) string) int {
	if selected == t("layer.fn") {
		return 1
	}
	return 0
}

func parseMatrixIndex(selected string) (int, bool) {
	if len(selected) < 3 {
		return 0, false
	}
	index, err := strconv.Atoi(selected[:3])
	return index, err == nil && index >= 0 && index < protocol.MaxMatrixKeys
}
