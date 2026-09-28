package ui

import (
	"context"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/joseferrao/ducky-drv/internal/protocol"
)

type actuationControls struct {
	root      fyne.CanvasObject
	key       *widget.Select
	all       *widget.Check
	point     *widget.Slider
	release   *widget.Slider
	rapid     *widget.Check
	mode      *widget.Label
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
	controls.key.OnChanged = func(string) { showActuationSelection(&controls) }

	load := widget.NewButtonWithIcon(t("action.load"), theme.DownloadIcon(), func() {
		u.run(func(ctx context.Context) (func(), error) {
			settings, err := u.client.Actuation(ctx)
			if err != nil {
				return nil, err
			}
			return func() {
				controls.settings = settings
				showActuationSelection(&controls)
				u.detail.SetText(t("status.loaded"))
			}, nil
		})
	})
	apply := widget.NewButtonWithIcon(t("action.apply"), theme.ConfirmIcon(), func() {
		setting := protocol.ActuationSetting{
			ActuationMM: controls.point.Value, ReleaseMM: controls.release.Value,
		}
		if controls.rapid.Checked {
			setting.Mode = 1
		}
		all := controls.all.Checked
		index, _ := parseMatrixIndex(controls.key.Selected)
		existing := append([]protocol.ActuationSetting(nil), controls.settings...)
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
					controls.settings = updated
					u.detail.SetText(t("status.saved"))
				}, nil
			}
			settings := existing
			if len(settings) != protocol.MaxMatrixKeys {
				var err error
				settings, err = u.client.Actuation(ctx)
				if err != nil {
					return nil, err
				}
			}
			settings[index] = setting
			if err := u.client.SetActuation(ctx, settings); err != nil {
				return nil, err
			}
			return func() {
				controls.settings = settings
				showActuationSelection(&controls)
				u.detail.SetText(t("status.saved"))
			}, nil
		})
	})
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
					controls.settings = settings
					showActuationSelection(&controls)
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
		widget.NewFormItem(t("actuation.point"), millimetreSlider(controls.point)),
		widget.NewFormItem(t("release.distance"), millimetreSlider(controls.release)),
		widget.NewFormItem("", controls.rapid),
	)
	description := widget.NewLabel(t("actuation.description"))
	description.Wrapping = fyne.TextWrapWord
	controls.root = container.NewBorder(
		container.NewVBox(description, widget.NewSeparator()),
		container.NewHBox(load, apply, reset), nil, nil,
		container.NewPadded(widget.NewCard(t("tab.actuation"), "0.1–3.5 mm", form)),
	)
	return controls
}

func millimetreSlider(slider *widget.Slider) fyne.CanvasObject {
	value := widget.NewLabel(formatMM(slider.Value))
	value.Alignment = fyne.TextAlignTrailing
	slider.OnChanged = func(number float64) { value.SetText(formatMM(number)) }
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
