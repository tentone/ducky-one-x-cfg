package ui

import (
	"context"
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/tentone/ducky-drv/internal/protocol"
)

type actuationControls struct {
	root      fyne.CanvasObject
	key       *widget.Select
	all       *widget.Check
	point     *widget.Slider
	release   *widget.Slider
	rapid     *widget.Check
	mode      *widget.Label
	keyboard  *KeyboardView
	settings  []protocol.ActuationSetting
	updating  bool
	modeMPT   string
	modeRT    string
	modeFixed string
}

func (u *UI) buildActuation() actuationControls {
	t := u.i18n.T
	controls := actuationControls{
		modeMPT: t("actuation.mpt"), modeRT: t("actuation.rapid"), modeFixed: t("actuation.fixed"),
	}
	controls.key = widget.NewSelect(protocol.MatrixKeyOptions(), nil)
	controls.key.SetSelected(protocol.MatrixKeyOptions()[0])
	controls.keyboard = NewKeyboardView(func(index int) {
		controls.key.SetSelected(matrixOption(index))
		controls.keyboard.Select(index)
		showActuationSelection(&controls)
	})
	controls.all = widget.NewCheck(t("selection.all"), func(checked bool) {
		if checked {
			controls.key.Disable()
		} else {
			controls.key.Enable()
		}
	})
	controls.point = widget.NewSlider(0.1, 3.5)
	controls.point.Step = 0.1
	controls.point.SetValue(1.0)
	controls.release = widget.NewSlider(0.1, 3.5)
	controls.release.Step = 0.1
	controls.release.SetValue(1.0)
	controls.rapid = widget.NewCheck(t("rapid.trigger"), nil)
	controls.mode = widget.NewLabel("—")
	controls.key.OnChanged = func(value string) {
		if index, ok := parseMatrixIndex(value); ok {
			controls.keyboard.Select(index)
		}
		showActuationSelection(&controls)
	}

	load := widget.NewButtonWithIcon(t("action.load"), theme.DownloadIcon(), func() {
		u.run(func(ctx context.Context) (func(), error) {
			settings, err := u.client.Actuation(ctx)
			if err != nil {
				return nil, err
			}
			return func() {
				u.setActuationSettings(&controls, settings)
				u.detail.SetText(t("status.loaded"))
			}, nil
		})
	})
	writeActuation := func(setting protocol.ActuationSetting, all bool, index int) {
		u.run(func(ctx context.Context) (func(), error) {
			if all {
				if err := u.client.SetAllActuation(ctx, setting); err != nil {
					return nil, err
				}
				updated := make([]protocol.ActuationSetting, protocol.MaxMatrixKeys)
				for i := range updated {
					updated[i] = setting
				}
				return func() {
					u.setActuationSettings(&controls, updated)
					u.detail.SetText(t("status.saved"))
				}, nil
			}
			settings, err := u.client.Actuation(ctx)
			if err != nil {
				return nil, err
			}
			settings[index] = setting
			if err := u.client.SetActuation(ctx, settings); err != nil {
				return nil, err
			}
			return func() {
				u.setActuationSettings(&controls, settings)
				u.detail.SetText(t("status.saved"))
			}, nil
		})
	}
	actuationSelection := func() (protocol.ActuationSetting, bool, int) {
		setting := protocol.ActuationSetting{
			ActuationMM: controls.point.Value, ReleaseMM: controls.release.Value,
		}
		if controls.rapid.Checked {
			setting.Mode = 1
		}
		index, _ := parseMatrixIndex(controls.key.Selected)
		return setting, controls.all.Checked, index
	}
	applyActuation := func() {
		setting, all, index := actuationSelection()
		writeActuation(setting, all, index)
	}
	apply := widget.NewButtonWithIcon(t("action.apply"), theme.ConfirmIcon(), applyActuation)
	autoApply := func() {
		if !controls.updating {
			setting, all, index := actuationSelection()
			feature := fmt.Sprintf("actuation:%d", index)
			if all {
				feature = "actuation:all"
			}
			u.scheduleAutoSync(feature, func() { writeActuation(setting, all, index) })
		}
	}
	controls.rapid.OnChanged = func(bool) { autoApply() }
	reset := widget.NewButtonWithIcon(t("action.reset"), theme.DeleteIcon(), func() {
		u.confirmReset(t("tab.actuation"), func() {
			u.run(func(ctx context.Context) (func(), error) {
				if err := u.client.ResetActuation(ctx); err != nil {
					return nil, err
				}
				settings, err := u.client.Actuation(ctx)
				if err != nil {
					return nil, err
				}
				return func() {
					u.setActuationSettings(&controls, settings)
					u.detail.SetText(t("status.reset"))
				}, nil
			})
		})
	})
	reset.Importance = widget.DangerImportance

	form := widget.NewForm(
		widget.NewFormItem(t("key"), controls.key),
		widget.NewFormItem("", controls.all),
		widget.NewFormItem(t("current"), controls.mode),
		widget.NewFormItem(t("actuation.point"), millimetreSlider(controls.point, func(float64) { autoApply() })),
		widget.NewFormItem(t("release.distance"), millimetreSlider(controls.release, func(float64) { autoApply() })),
		widget.NewFormItem("", controls.rapid),
	)
	description := widget.NewLabel(t("actuation.description"))
	description.Wrapping = fyne.TextWrapWord
	keyboardCard := widget.NewCard(t("tab.actuation"), "", controls.keyboard.CanvasObject())
	editorCard := widget.NewCard(t("current"), "0.1–3.5 mm", form)
	controls.root = container.NewBorder(
		container.NewVBox(description, widget.NewSeparator()),
		container.NewHBox(load, apply, reset), nil, nil,
		container.NewVScroll(container.NewPadded(container.NewVBox(keyboardCard, editorCard))),
	)
	return controls
}

func (u *UI) setActuationSettings(controls *actuationControls, settings []protocol.ActuationSetting) {
	u.withoutAutoSync(func() {
		controls.settings = settings
		u.updateActuationKeyboard(controls)
		showActuationSelection(controls)
	})
}

func (u *UI) updateActuationKeyboard(controls *actuationControls) {
	controls.keyboard.ForEach(func(index int, key *keyboardKey) {
		if index >= len(controls.settings) {
			key.SetOverlay("—")
			key.SetColor(theme.InputBackgroundColor())
			return
		}
		setting := controls.settings[index]
		key.SetOverlay(formatFloat(setting.ActuationMM))
		if setting.ControlledByMPT() {
			key.SetColor(color.NRGBA{R: 131, G: 73, B: 163, A: 255})
			return
		}
		amount := (setting.ActuationMM - 0.1) / 3.4
		if amount < 0 {
			amount = 0
		}
		if amount > 1 {
			amount = 1
		}
		near := color.NRGBA{R: 41, G: 145, B: 101, A: 255}
		far := color.NRGBA{R: 210, G: 115, B: 52, A: 255}
		key.SetColor(blendColor(near, far, amount))
	})
}

func millimetreSlider(slider *widget.Slider, changed ...func(float64)) fyne.CanvasObject {
	value := widget.NewLabel(formatMM(slider.Value))
	value.Alignment = fyne.TextAlignTrailing
	slider.OnChanged = func(number float64) {
		value.SetText(formatMM(number))
		for _, callback := range changed {
			callback(number)
		}
	}
	return container.NewBorder(nil, nil, nil, container.NewGridWrap(fyne.NewSize(64, 32), value), slider)
}

func formatMM(value float64) string { return formatFloat(value) + " mm" }

func formatFloat(value float64) string {
	return fmt.Sprintf("%.1f", value)
}

func showActuationSelection(controls *actuationControls) {
	if controls.updating {
		return
	}
	index, ok := parseMatrixIndex(controls.key.Selected)
	if !ok || index >= len(controls.settings) {
		controls.mode.SetText("—")
		return
	}
	setting := controls.settings[index]
	controls.updating = true
	controls.point.SetValue(setting.ActuationMM)
	if setting.ReleaseMM > 0 {
		controls.release.SetValue(setting.ReleaseMM)
	}
	controls.rapid.SetChecked(setting.RapidTrigger())
	controls.updating = false
	if setting.ControlledByMPT() {
		controls.mode.SetText(controls.modeMPT)
	} else if setting.RapidTrigger() {
		controls.mode.SetText(controls.modeRT)
	} else {
		controls.mode.SetText(controls.modeFixed)
	}
}
