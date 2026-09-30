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
	"github.com/tentone/ducky-drv/internal/protocol"
)

type lightingControls struct {
	root       fyne.CanvasObject
	effect     *widget.Select
	speed      *widget.Slider
	brightness *widget.Slider
	pattern    *widget.Select
	direction  *widget.Select
	red        byte
	green      byte
	blue       byte
	preview    *canvas.Rectangle
	keyboard   *KeyboardView
	animator   *LightingAnimator
	keyColors  map[int]color.NRGBA
	colorRow   fyne.CanvasObject
	speedRow   fyne.CanvasObject
	brightRow  fyne.CanvasObject
	patternRow fyne.CanvasObject
	directRow  fyne.CanvasObject
	paintRow   fyne.CanvasObject
	updating   bool
}

var lightingEffects = []struct {
	nameID string
	code   byte
}{
	{"effect.static", 13}, {"effect.breathing", 3}, {"effect.cycle", 15}, {"effect.reactive", 25},
	{"effect.ripple", 39}, {"effect.rainbow", 21}, {"effect.analog", 49},
	{"effect.custom", protocol.CustomStaticLightingEffect}, {"effect.off", 103},
}

func (u *UI) buildLighting() lightingControls {
	t := u.i18n.T
	controls := lightingControls{red: 234, green: 168, blue: 42, keyColors: make(map[int]color.NRGBA)}
	var autoApplyLighting func()
	controls.keyboard = NewKeyboardView(func(index int) {
		if controls.effect != nil && effectCode(controls.effect.Selected, t) == protocol.CustomStaticLightingEffect {
			controls.keyColors[index] = color.NRGBA{R: controls.red, G: controls.green, B: controls.blue, A: 255}
			controls.animator.SetCustomColors(controls.keyColors)
			if autoApplyLighting != nil {
				autoApplyLighting()
			}
			return
		}
		if controls.animator != nil {
			controls.animator.Press(index)
		}
	})
	controls.keyboard.SetTapFlash(false)
	controls.animator = NewLightingAnimator(controls.keyboard)

	effects := make([]string, len(lightingEffects))
	for i, effect := range lightingEffects {
		effects[i] = t(effect.nameID)
	}
	controls.effect = widget.NewSelect(effects, nil)
	controls.effect.SetSelected(effects[0])
	controls.speed = widget.NewSlider(10, 100)
	controls.speed.Step = 10
	controls.speed.SetValue(50)
	controls.brightness = widget.NewSlider(0, 100)
	controls.brightness.Step = 25
	controls.brightness.SetValue(100)
	controls.pattern = widget.NewSelect(
		[]string{t("pattern.rainbow"), t("pattern.yellow_red"), t("pattern.blue_red")}, nil,
	)
	controls.pattern.SetSelected(t("pattern.rainbow"))
	controls.direction = widget.NewSelect(
		[]string{t("direction.right"), t("direction.left"), t("direction.down"), t("direction.up")}, nil,
	)
	controls.direction.SetSelected(t("direction.right"))
	controls.preview = canvas.NewRectangle(color.NRGBA{R: controls.red, G: controls.green, B: controls.blue, A: 255})
	controls.preview.SetMinSize(fyne.NewSize(56, 30))

	var applyLighting func()
	updateAnimation := func() {
		if !controls.updating {
			controls.animator.Set(lightingSettingsFromControls(&controls, t))
			u.window.Canvas().Unfocus()
			if autoApplyLighting != nil {
				autoApplyLighting()
			}
		}
	}
	controls.effect.OnChanged = func(string) {
		if controls.updating {
			return
		}
		u.updateLightingVisibility(&controls)
		updateAnimation()
	}
	controls.pattern.OnChanged = func(string) { updateAnimation() }
	controls.direction.OnChanged = func(string) { updateAnimation() }

	chooseColor := widget.NewButtonWithIcon(t("color"), theme.ColorPaletteIcon(), func() {
		picker := dialog.NewColorPicker(t("color"), t("action.select"), func(selected color.Color) {
			controls.red, controls.green, controls.blue = rgbaBytes(selected)
			controls.preview.FillColor = color.NRGBA{R: controls.red, G: controls.green, B: controls.blue, A: 255}
			controls.preview.Refresh()
			if effectCode(controls.effect.Selected, t) == protocol.CustomStaticLightingEffect {
				return
			}
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
			var custom protocol.CustomLightingSettings
			if settings.Effect == protocol.CustomStaticLightingEffect {
				custom, err = u.client.CustomLighting(ctx)
				if err != nil {
					return nil, err
				}
				settings.Brightness = custom.Brightness
			}
			return func() {
				u.setLightingSettings(&controls, settings)
				if settings.Effect == protocol.CustomStaticLightingEffect {
					u.setCustomLighting(&controls, custom)
				}
				u.detail.SetText(t("status.loaded"))
			}, nil
		})
	})
	writeLighting := func(settings protocol.LightingSettings, custom protocol.CustomLightingSettings) {
		u.run(func(ctx context.Context) (func(), error) {
			if settings.Effect == protocol.CustomStaticLightingEffect {
				if err := u.client.SetCustomLighting(ctx, custom); err != nil {
					return nil, err
				}
			} else if err := u.client.SetLighting(ctx, settings); err != nil {
				return nil, err
			}
			return func() { u.detail.SetText(t("status.saved")) }, nil
		})
	}
	applyLighting = func() {
		writeLighting(lightingSettingsFromControls(&controls, t), customLightingFromControls(&controls))
	}
	autoApplyLighting = func() {
		settings := lightingSettingsFromControls(&controls, t)
		custom := customLightingFromControls(&controls)
		u.scheduleAutoSync("lighting", func() { writeLighting(settings, custom) })
	}
	apply := widget.NewButtonWithIcon(t("action.apply"), theme.ConfirmIcon(), applyLighting)

	colorControl := container.NewBorder(nil, nil, controls.preview, nil, chooseColor)
	controls.colorRow = lightingSettingRow(t("color"), colorControl)
	controls.brightRow = lightingSettingRow(t("brightness"), sliderValue(controls.brightness, "%", func(float64) { updateAnimation() }))
	controls.speedRow = lightingSettingRow(t("speed"), sliderValue(controls.speed, "%", func(float64) { updateAnimation() }))
	controls.patternRow = lightingSettingRow(t("pattern"), container.NewGridWrap(fyne.NewSize(210, 36), controls.pattern))
	controls.directRow = lightingSettingRow(t("direction"), container.NewGridWrap(fyne.NewSize(210, 36), controls.direction))
	paintHint := widget.NewLabel(t("lighting.paint_hint"))
	paintHint.Wrapping = fyne.TextWrapWord
	clearPaint := widget.NewButtonWithIcon(t("action.clear_keys"), theme.DeleteIcon(), func() {
		controls.keyColors = make(map[int]color.NRGBA)
		controls.animator.SetCustomColors(controls.keyColors)
		if autoApplyLighting != nil {
			autoApplyLighting()
		}
	})
	controls.paintRow = lightingSettingRow(t("lighting.paint"), container.NewBorder(nil, nil, nil, clearPaint, paintHint))
	effectRow := lightingSettingRow(t("effect"), container.NewGridWrap(fyne.NewSize(210, 36), controls.effect))

	description := widget.NewLabel(t("lighting.description"))
	description.Wrapping = fyne.TextWrapWord
	keyboardCard := widget.NewCard(t("tab.lighting"), "", controls.keyboard.CanvasObject())
	settingsCard := widget.NewCard(t("effect"), "", container.NewVBox(
		effectRow, controls.colorRow, controls.paintRow, controls.brightRow, controls.speedRow, controls.patternRow, controls.directRow,
	))
	controls.root = container.NewBorder(
		container.NewVBox(description, widget.NewSeparator()),
		container.NewHBox(load, apply), nil, nil,
		container.NewVScroll(container.NewPadded(container.NewVBox(keyboardCard, settingsCard))),
	)
	u.updateLightingVisibility(&controls)
	updateAnimation()
	return controls
}

func lightingSettingRow(label string, control fyne.CanvasObject) fyne.CanvasObject {
	name := widget.NewLabel(label)
	return container.NewBorder(nil, nil, container.NewGridWrap(fyne.NewSize(145, 36), name), nil, control)
}

func (u *UI) updateLightingVisibility(controls *lightingControls) {
	effect := effectCode(controls.effect.Selected, u.i18n.T)
	setVisible(controls.colorRow, effect != 15 && effect != 21 && effect != 103)
	setVisible(controls.brightRow, effect != 103)
	setVisible(controls.speedRow, effect == 3 || effect == 15 || effect == 25 || effect == 39 || effect == 21)
	setVisible(controls.patternRow, effect == 21)
	setVisible(controls.directRow, effect == 21)
	setVisible(controls.paintRow, effect == protocol.CustomStaticLightingEffect)
}

func setVisible(object fyne.CanvasObject, visible bool) {
	if visible {
		object.Show()
	} else {
		object.Hide()
	}
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
	settings := protocol.LightingSettings{
		Effect: effectCode(controls.effect.Selected, t), Speed: int(controls.speed.Value),
		Red: controls.red, Green: controls.green, Blue: controls.blue,
		Brightness: int(controls.brightness.Value),
	}
	if settings.Effect == 15 {
		// The firmware ignores user color for Color Cycle; these are the web tool's canonical bytes.
		settings.Red, settings.Green, settings.Blue = 167, 45, 45
	}
	if settings.Effect == 21 {
		settings.Variant = rainbowPatternCode(controls.pattern.Selected, t)
		settings.ColorMode = rainbowDirectionCode(controls.direction.Selected, t)
	}
	return settings
}

func customLightingFromControls(controls *lightingControls) protocol.CustomLightingSettings {
	settings := protocol.CustomLightingSettings{Brightness: int(controls.brightness.Value)}
	settings.Colors = make([]protocol.KeyColor, 0, len(controls.keyColors))
	for index, keyColor := range controls.keyColors {
		settings.Colors = append(settings.Colors, protocol.KeyColor{
			Key: byte(index), Red: keyColor.R, Green: keyColor.G, Blue: keyColor.B,
		})
	}
	return settings
}

func (u *UI) setCustomLighting(controls *lightingControls, settings protocol.CustomLightingSettings) {
	u.withoutAutoSync(func() {
		controls.keyColors = make(map[int]color.NRGBA, len(settings.Colors))
		for _, keyColor := range settings.Colors {
			controls.keyColors[int(keyColor.Key)] = color.NRGBA{
				R: keyColor.Red, G: keyColor.Green, B: keyColor.Blue, A: 255,
			}
		}
		controls.animator.SetCustomColors(controls.keyColors)
	})
}

func (u *UI) setLightingSettings(controls *lightingControls, settings protocol.LightingSettings) {
	u.withoutAutoSync(func() {
		controls.updating = true
		controls.effect.SetSelected(effectName(settings.Effect, u.i18n.T))
		if settings.Speed < 10 {
			settings.Speed = 10
		}
		controls.speed.SetValue(float64(settings.Speed))
		controls.brightness.SetValue(float64(settings.Brightness))
		controls.pattern.SetSelected(rainbowPatternName(settings.Variant, u.i18n.T))
		controls.direction.SetSelected(rainbowDirectionName(settings.ColorMode, u.i18n.T))
		controls.red, controls.green, controls.blue = settings.Red, settings.Green, settings.Blue
		controls.preview.FillColor = color.NRGBA{R: settings.Red, G: settings.Green, B: settings.Blue, A: 255}
		controls.preview.Refresh()
		controls.updating = false
	})
	u.updateLightingVisibility(controls)
	controls.animator.Set(lightingSettingsFromControls(controls, u.i18n.T))
}

func rainbowPatternCode(name string, t func(string) string) byte {
	switch name {
	case t("pattern.yellow_red"):
		return 1
	case t("pattern.blue_red"):
		return 2
	default:
		return 0
	}
}

func rainbowPatternName(code byte, t func(string) string) string {
	switch code {
	case 1:
		return t("pattern.yellow_red")
	case 2:
		return t("pattern.blue_red")
	default:
		return t("pattern.rainbow")
	}
}

func rainbowDirectionCode(name string, t func(string) string) byte {
	switch name {
	case t("direction.left"):
		return 1
	case t("direction.down"):
		return 2
	case t("direction.up"):
		return 3
	default:
		return 0
	}
}

func rainbowDirectionName(code byte, t func(string) string) string {
	switch code {
	case 1:
		return t("direction.left")
	case 2:
		return t("direction.down")
	case 3:
		return t("direction.up")
	default:
		return t("direction.right")
	}
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
