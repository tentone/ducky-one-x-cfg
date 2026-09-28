package ui

import (
	"context"
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/joseferrao/ducky-drv/internal/protocol"
)

type lightingControls struct {
	root       fyne.CanvasObject
	effect     *widget.Select
	speed      *widget.Slider
	brightness *widget.Slider
	variant    *widget.Slider
	red        byte
	green      byte
	blue       byte
	preview    *canvas.Rectangle
	keyboard   *KeyboardView
	animator   *LightingAnimator
}

var lightingEffects = []struct {
	nameID string
	code   byte
}{
	{"effect.static", 13}, {"effect.breathing", 3}, {"effect.cycle", 15}, {"effect.reactive", 25},
	{"effect.ripple", 39}, {"effect.rainbow", 21}, {"effect.analog", 49}, {"effect.off", 103},
}

func (u *UI) buildLighting() lightingControls {
	t := u.i18n.T
	controls := lightingControls{red: 234, green: 168, blue: 42}
	controls.keyboard = NewKeyboardView(nil)
	controls.animator = NewLightingAnimator(controls.keyboard)
	effects := make([]string, len(lightingEffects))
	for i, effect := range lightingEffects {
		effects[i] = t(effect.nameID)
	}
	controls.effect = widget.NewSelect(effects, nil)
	controls.effect.SetSelected(effects[0])
	controls.speed = widget.NewSlider(0, 100)
	controls.speed.Step = 10
	controls.speed.SetValue(50)
	controls.brightness = widget.NewSlider(0, 100)
	controls.brightness.Step = 25
	controls.brightness.SetValue(100)
	controls.variant = widget.NewSlider(0, 8)
	controls.variant.Step = 1
	controls.preview = canvas.NewRectangle(color.NRGBA{R: controls.red, G: controls.green, B: controls.blue, A: 255})
	controls.preview.SetMinSize(fyne.NewSize(56, 32))
	updateAnimation := func() { controls.animator.Set(lightingSettingsFromControls(&controls, t)) }
	controls.effect.OnChanged = func(string) { updateAnimation() }
	chooseColor := widget.NewButtonWithIcon(t("color"), theme.ColorPaletteIcon(), func() {
		picker := dialog.NewColorPicker(t("color"), t("action.select"), func(selected color.Color) {
			controls.red, controls.green, controls.blue = rgbaBytes(selected)
			controls.preview.FillColor = color.NRGBA{R: controls.red, G: controls.green, B: controls.blue, A: 255}
			controls.preview.Refresh()
			updateAnimation()
		}, u.window)
		picker.Advanced = true
		picker.Show()
	})

	load := widget.NewButtonWithIcon(t("action.load"), theme.DownloadIcon(), func() {
		u.run(func(ctx context.Context) (func(), error) {
			settings, err := u.client.Lighting(ctx)
			if err != nil {
				return nil, err
			}
			return func() {
				u.setLightingSettings(&controls, settings)
				u.detail.SetText(t("status.loaded"))
			}, nil
		})
	})
	apply := widget.NewButtonWithIcon(t("action.apply"), theme.ConfirmIcon(), func() {
		settings := lightingSettingsFromControls(&controls, t)
		u.run(func(ctx context.Context) (func(), error) {
			if err := u.client.SetLighting(ctx, settings); err != nil {
				return nil, err
			}
			return func() { u.detail.SetText(t("status.saved")) }, nil
		})
	})

	form := widget.NewForm(
		widget.NewFormItem(t("effect"), controls.effect),
		widget.NewFormItem(t("color"), container.NewBorder(nil, nil, controls.preview, nil, chooseColor)),
		widget.NewFormItem(t("brightness"), sliderValue(controls.brightness, "%", func(float64) { updateAnimation() })),
		widget.NewFormItem(t("speed"), sliderValue(controls.speed, "%", func(float64) { updateAnimation() })),
		widget.NewFormItem(t("direction"), sliderValue(controls.variant, "", func(float64) { updateAnimation() })),
	)
	description := widget.NewLabel(t("lighting.description"))
	description.Wrapping = fyne.TextWrapWord
	keyboardCard := widget.NewCard(t("tab.lighting"), "", controls.keyboard.CanvasObject())
	settingsCard := widget.NewCard(t("effect"), "", form)
	controls.root = container.NewBorder(
		container.NewVBox(description, widget.NewSeparator()),
		container.NewHBox(load, apply), nil, nil,
		container.NewVScroll(container.NewPadded(container.NewVBox(keyboardCard, settingsCard))),
	)
	updateAnimation()
	return controls
}

func sliderValue(slider *widget.Slider, suffix string, changed ...func(float64)) fyne.CanvasObject {
	value := widget.NewLabel(fmt.Sprintf("%.0f%s", slider.Value, suffix))
	value.Alignment = fyne.TextAlignTrailing
	slider.OnChanged = func(number float64) {
		value.SetText(fmt.Sprintf("%.0f%s", number, suffix))
		for _, callback := range changed {
			callback(number)
		}
	}
	return container.NewBorder(nil, nil, nil, container.NewGridWrap(fyne.NewSize(52, 32), value), slider)
}

func lightingSettingsFromControls(controls *lightingControls, t func(string) string) protocol.LightingSettings {
	return protocol.LightingSettings{
		Effect: effectCode(controls.effect.Selected, t), Speed: int(controls.speed.Value),
		Red: controls.red, Green: controls.green, Blue: controls.blue,
		Brightness: int(controls.brightness.Value), Variant: byte(controls.variant.Value),
	}
}

func (u *UI) setLightingSettings(controls *lightingControls, settings protocol.LightingSettings) {
	controls.effect.SetSelected(effectName(settings.Effect, u.i18n.T))
	controls.speed.SetValue(float64(settings.Speed))
	controls.brightness.SetValue(float64(settings.Brightness))
	controls.variant.SetValue(float64(settings.Variant))
	controls.red, controls.green, controls.blue = settings.Red, settings.Green, settings.Blue
	controls.preview.FillColor = color.NRGBA{R: settings.Red, G: settings.Green, B: settings.Blue, A: 255}
	controls.preview.Refresh()
	controls.animator.Set(settings)
}

func effectName(code byte, t func(string) string) string {
	for _, effect := range lightingEffects {
		if effect.code == code {
			return t(effect.nameID)
		}
	}
	return fmt.Sprintf("Unknown (0x%02x)", code)
}

func effectCode(name string, t func(string) string) byte {
	for _, effect := range lightingEffects {
		if t(effect.nameID) == name {
			return effect.code
		}
	}
	return 13
}
