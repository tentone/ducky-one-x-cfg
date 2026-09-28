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

type mptStageControls struct {
	press   *widget.Slider
	release *widget.Slider
	output  *widget.Select
}

type mptControls struct {
	root     fyne.CanvasObject
	preset   *widget.Select
	stages   [4]mptStageControls
	updating bool
}

func (u *UI) buildMPT() mptControls {
	t := u.i18n.T
	controls := mptControls{}
	presets := make([]string, protocol.MPTPresets)
	for i := range presets {
		presets[i] = fmt.Sprintf("MPT%d", i+1)
	}
	controls.preset = widget.NewSelect(presets, nil)
	controls.preset.SetSelected(presets[0])

	outputs := append([]string{t("output.disabled")}, protocol.KeyOptions(true)...)
	stageCards := make([]fyne.CanvasObject, 4)
	var autoApply func()
	for i := range controls.stages {
		stage := &controls.stages[i]
		stage.press = widget.NewSlider(0.1, 3.5)
		stage.press.Step = 0.1
		stage.press.SetValue(float64(i+1) * 0.5)
		stage.release = widget.NewSlider(0.1, 3.5)
		stage.release.Step = 0.1
		stage.release.SetValue(float64(i+1) * 0.5)
		stage.output = widget.NewSelect(outputs, nil)
		stage.output.SetSelected(t("output.disabled"))
		stage.output.OnChanged = func(string) {
			if autoApply != nil {
				autoApply()
			}
		}
		form := widget.NewForm(
			widget.NewFormItem(t("press.distance"), millimetreSlider(stage.press, func(float64) {
				if autoApply != nil {
					autoApply()
				}
			})),
			widget.NewFormItem(t("release.point"), millimetreSlider(stage.release, func(float64) {
				if autoApply != nil {
					autoApply()
				}
			})),
			widget.NewFormItem(t("output.key"), stage.output),
		)
		stageCards[i] = widget.NewCard(fmt.Sprintf("%s %d", t("mpt.stage"), i+1), "", form)
	}

	load := widget.NewButtonWithIcon(t("action.load"), theme.DownloadIcon(), func() {
		preset := parsePreset(controls.preset.Selected)
		u.run(func(ctx context.Context) (func(), error) {
			stages, err := u.client.MPT(ctx, preset)
			if err != nil {
				return nil, err
			}
			return func() {
				u.setMPTStageControls(&controls, stages)
				u.detail.SetText(t("status.loaded"))
			}, nil
		})
	})
	mptSelection := func() (int, []protocol.MPTStage) {
		preset := parsePreset(controls.preset.Selected)
		stages := make([]protocol.MPTStage, 4)
		for i, controls := range controls.stages {
			stages[i].PressMM = controls.press.Value
			stages[i].ReleaseMM = controls.release.Value
			if controls.output.Selected != t("output.disabled") {
				stages[i].Output, _ = protocol.KeyCode(controls.output.Selected)
				stages[i].Mouse = stages[i].Output >= 244 && stages[i].Output <= 246
			}
		}
		return preset, stages
	}
	writeMPT := func(preset int, stages []protocol.MPTStage) {
		u.run(func(ctx context.Context) (func(), error) {
			if err := u.client.SetMPT(ctx, preset, stages); err != nil {
				return nil, err
			}
			return func() { u.detail.SetText(t("status.saved")) }, nil
		})
	}
	applyMPT := func() {
		preset, stages := mptSelection()
		writeMPT(preset, stages)
	}
	apply := widget.NewButtonWithIcon(t("action.apply"), theme.ConfirmIcon(), applyMPT)
	autoApply = func() {
		if !controls.updating {
			preset, stages := mptSelection()
			u.scheduleAutoSync(fmt.Sprintf("mpt:%d", preset), func() { writeMPT(preset, stages) })
		}
	}
	reset := widget.NewButtonWithIcon(t("action.reset"), theme.DeleteIcon(), func() {
		u.confirmReset(t("tab.mpt"), func() {
			u.run(func(ctx context.Context) (func(), error) {
				if err := u.client.ResetMPT(ctx); err != nil {
					return nil, err
				}
				return func() {
					u.withoutAutoSync(func() {
						for i := range controls.stages {
							controls.stages[i].output.SetSelected(t("output.disabled"))
						}
					})
					u.detail.SetText(t("status.reset"))
				}, nil
			})
		})
	})
	reset.Importance = widget.DangerImportance

	description := widget.NewLabel(t("mpt.description"))
	description.Wrapping = fyne.TextWrapWord
	toolbar := container.NewBorder(nil, nil, container.NewHBox(widget.NewLabel(t("mpt.preset")), controls.preset), nil, container.NewHBox(load, apply, reset))
	controls.root = container.NewBorder(
		container.NewVBox(description, widget.NewSeparator(), toolbar),
		nil, nil, nil,
		container.NewVScroll(container.NewPadded(container.NewGridWithColumns(2, stageCards...))),
	)
	return controls
}

func (u *UI) setMPTStageControls(controls *mptControls, stages []protocol.MPTStage) {
	u.withoutAutoSync(func() {
		controls.updating = true
		defer func() { controls.updating = false }()
		for i, value := range stages {
			if i >= len(controls.stages) {
				break
			}
			if value.PressMM > 0 {
				controls.stages[i].press.SetValue(value.PressMM)
			}
			if value.ReleaseMM > 0 {
				controls.stages[i].release.SetValue(value.ReleaseMM)
			}
			if value.Output == 0 {
				controls.stages[i].output.SetSelected(u.i18n.T("output.disabled"))
			} else {
				controls.stages[i].output.SetSelected(protocol.KeyName(value.Output))
			}
		}
	})
}

func parsePreset(value string) int {
	n, err := strconv.Atoi(strings.TrimPrefix(value, "MPT"))
	if err != nil || n < 1 {
		return 0
	}
	return n - 1
}
