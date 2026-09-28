package ui

import (
	"context"
	"fmt"
	"image/color"
	"log"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/joseferrao/ducky-drv/internal/device"
	"github.com/joseferrao/ducky-drv/internal/i18n"
	"github.com/joseferrao/ducky-drv/internal/protocol"
)

const preferenceTheme = "appearance.theme"

type UI struct {
	app     fyne.App
	window  fyne.Window
	i18n    *i18n.Catalog
	manager *device.Manager
	client  *protocol.Client

	devices       []device.Descriptor
	deviceSelect  *widget.Select
	connectButton *widget.Button
	profileSelect *widget.Select
	status        *widget.Label
	detail        *widget.Label
	busy          bool
	updating      bool
	autoConnect   bool
	done          chan struct{}

	keys      keyControls
	lighting  lightingControls
	actuation actuationControls
	mpt       mptControls
	macros    macroControls
}

func Run() error {
	manager, err := device.NewManager()
	if err != nil {
		return err
	}
	application := app.NewWithID("io.ducky.one-x.configurator")
	u := &UI{
		app: application, window: application.NewWindow("Ducky One X Configurator"),
		i18n: i18n.New(), manager: manager, autoConnect: true, done: make(chan struct{}),
	}
	u.window.SetOnClosed(func() {
		close(u.done)
		if u.lighting.animator != nil {
			u.lighting.animator.Stop()
		}
		if err := manager.Close(); err != nil {
			log.Printf("close HID manager: %v", err)
		}
	})
	u.build()
	u.applySavedTheme()
	u.window.Resize(fyne.NewSize(1120, 760))
	u.window.SetContent(u.content())
	u.refreshDevices()
	u.startAutoDiscovery()
	u.window.ShowAndRun()
	return nil
}

func (u *UI) build() {
	t := u.i18n.T
	u.status = widget.NewLabel(t("connection.disconnected"))
	u.status.TextStyle = fyne.TextStyle{Bold: true}
	u.detail = widget.NewLabel(t("connection.wired"))
	u.detail.Wrapping = fyne.TextWrapWord

	u.deviceSelect = widget.NewSelect(nil, nil)
	u.deviceSelect.PlaceHolder = t("device")
	u.connectButton = widget.NewButtonWithIcon(t("action.connect"), theme.MediaPlayIcon(), u.toggleConnection)
	u.profileSelect = widget.NewSelect([]string{t("profile.1"), t("profile.2")}, u.changeProfile)
	u.profileSelect.Disable()

	u.keys = u.buildKeys()
	u.lighting = u.buildLighting()
	u.actuation = u.buildActuation()
	u.mpt = u.buildMPT()
	u.macros = u.buildMacros()
}

func (u *UI) content() fyne.CanvasObject {
	t := u.i18n.T
	title := widget.NewLabel(t("app.title"))
	title.TextStyle = fyne.TextStyle{Bold: true}

	refresh := widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), func() {
		u.autoConnect = true
		u.refreshDevices()
	})
	refresh.Importance = widget.LowImportance
	themeSelect := widget.NewSelect(
		[]string{t("theme.system"), t("theme.light"), t("theme.dark")},
		u.changeTheme,
	)
	themeSelect.SetSelected(u.themeLabel(u.app.Preferences().StringWithFallback(preferenceTheme, "system")))
	language := widget.NewSelect([]string{"English"}, func(string) {})
	language.SetSelected("English")

	preferences := container.NewHBox(
		widget.NewLabel(t("profile")), compactControl(u.profileSelect, 112),
		widget.NewLabel(t("theme")), compactControl(themeSelect, 96),
		compactControl(language, 88),
	)
	deviceControls := container.NewHBox(compactControl(u.deviceSelect, 220), refresh, u.connectButton)
	header := container.NewBorder(nil, nil, title, preferences, deviceControls)

	tabs := container.NewAppTabs(
		container.NewTabItemWithIcon(t("tab.keys"), theme.ComputerIcon(), u.keys.root),
		container.NewTabItemWithIcon(t("tab.lighting"), theme.VisibilityIcon(), u.lighting.root),
		container.NewTabItemWithIcon(t("tab.actuation"), theme.SettingsIcon(), u.actuation.root),
		container.NewTabItemWithIcon(t("tab.mpt"), theme.StorageIcon(), u.mpt.root),
		container.NewTabItemWithIcon(t("tab.macros"), theme.ContentAddIcon(), u.macros.root),
	)
	tabs.SetTabLocation(container.TabLocationLeading)

	statusBar := container.NewBorder(nil, nil, themeStatusIcon(), nil, container.NewVBox(u.status, u.detail))
	return container.NewBorder(container.NewVBox(header, widget.NewSeparator()), statusBar, nil, nil, tabs)
}

func compactControl(object fyne.CanvasObject, width float32) fyne.CanvasObject {
	return container.NewGridWrap(fyne.NewSize(width, 36), object)
}

func themeStatusIcon() fyne.CanvasObject {
	icon := widget.NewIcon(theme.InfoIcon())
	return container.NewCenter(icon)
}

func (u *UI) refreshDevices() {
	if u.busy || u.client != nil {
		return
	}
	devices, err := u.manager.List()
	if err != nil {
		u.showError(err)
		return
	}
	u.devices = devices
	options := make([]string, len(devices))
	for i, descriptor := range devices {
		options[i] = descriptor.DisplayName()
	}
	u.deviceSelect.SetOptions(options)
	if len(options) > 0 {
		u.deviceSelect.SetSelected(options[0])
		u.detail.SetText(fmt.Sprintf(u.i18n.T("connection.found"), len(options)))
		if u.autoConnect {
			u.connectSelected()
		}
	} else {
		u.deviceSelect.ClearSelected()
		u.detail.SetText(u.i18n.T("connection.wired"))
	}
}

func (u *UI) toggleConnection() {
	if u.client != nil {
		u.autoConnect = false
		if err := u.manager.Disconnect(); err != nil {
			u.showError(err)
		}
		u.client = nil
		u.clearLoadedState()
		u.profileSelect.Disable()
		u.connectButton.SetText(u.i18n.T("action.connect"))
		u.connectButton.SetIcon(theme.MediaPlayIcon())
		u.status.SetText(u.i18n.T("connection.disconnected"))
		u.detail.SetText(u.i18n.T("connection.wired"))
		return
	}
	u.autoConnect = true
	u.refreshDevices()
}

func (u *UI) connectSelected() {
	if u.client != nil || u.busy {
		return
	}
	selected := u.deviceSelect.Selected
	index := -1
	for i, descriptor := range u.devices {
		if descriptor.DisplayName() == selected {
			index = i
			break
		}
	}
	if index < 0 {
		dialog.ShowInformation(u.i18n.T("action.connect"), u.i18n.T("connection.wired"), u.window)
		return
	}
	descriptor := u.devices[index]
	u.detail.SetText(fmt.Sprintf(u.i18n.T("connection.opening"), descriptor.DisplayName()))
	u.setBusy(true, u.i18n.T("connection.loading"))
	go func() {
		session, err := u.manager.Connect(descriptor.Path)
		if err != nil {
			fyne.Do(func() {
				u.autoConnect = false
				u.finish(nil, err)
			})
			return
		}
		client := protocol.NewClient(session)
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		metadata, metadataErr := client.Metadata(ctx)
		var profile int
		var profileErr error
		var configuration configurationSnapshot
		var configurationErr error
		if metadataErr == nil {
			profile, profileErr = client.ActiveProfile(ctx)
			configuration, configurationErr = u.readConfiguration(ctx, client)
		}
		fyne.Do(func() {
			if metadataErr != nil {
				_ = u.manager.Disconnect()
				u.autoConnect = false
				u.finish(nil, metadataErr)
				return
			}
			u.client = client
			u.profileSelect.Enable()
			u.updating = true
			if profileErr == nil && profile == 1 {
				u.profileSelect.SetSelected(u.i18n.T("profile.2"))
			} else {
				u.profileSelect.SetSelected(u.i18n.T("profile.1"))
			}
			u.updating = false
			u.connectButton.SetText(u.i18n.T("action.disconnect"))
			u.connectButton.SetIcon(theme.MediaStopIcon())
			if configurationErr == nil {
				u.applyConfiguration(configuration)
			}
			u.detail.SetText(fmt.Sprintf(u.i18n.T("device.summary"), descriptor.DisplayName(), metadata.Firmware()))
			u.finish(nil, nil)
			if profileErr != nil {
				u.showError(profileErr)
			} else if configurationErr != nil {
				u.showError(configurationErr)
			}
		})
	}()
}

type configurationSnapshot struct {
	layer     int
	mapping   []protocol.Assignment
	lighting  protocol.LightingSettings
	actuation []protocol.ActuationSetting
	mpt       []protocol.MPTStage
	macros    []protocol.MacroAction
}

func (u *UI) readConfiguration(ctx context.Context, client *protocol.Client) (configurationSnapshot, error) {
	var snapshot configurationSnapshot
	var err error
	if snapshot.layer, err = client.ActiveLayer(ctx); err != nil {
		return snapshot, fmt.Errorf("read active layer: %w", err)
	}
	if snapshot.layer < 0 || snapshot.layer > 1 {
		snapshot.layer = 0
	}
	if err = client.SetLayer(ctx, snapshot.layer); err != nil {
		return snapshot, fmt.Errorf("select active layer: %w", err)
	}
	if snapshot.mapping, err = client.KeyMap(ctx, snapshot.layer); err != nil {
		return snapshot, fmt.Errorf("read key settings: %w", err)
	}
	if snapshot.lighting, err = client.Lighting(ctx); err != nil {
		return snapshot, fmt.Errorf("read lighting: %w", err)
	}
	if snapshot.actuation, err = client.Actuation(ctx); err != nil {
		return snapshot, fmt.Errorf("read actuation: %w", err)
	}
	if snapshot.mpt, err = client.MPT(ctx, 0); err != nil {
		return snapshot, fmt.Errorf("read MPT1: %w", err)
	}
	if snapshot.macros, err = client.Macro(ctx, 1); err != nil {
		return snapshot, fmt.Errorf("read macro M1: %w", err)
	}
	return snapshot, nil
}

func (u *UI) applyConfiguration(snapshot configurationSnapshot) {
	u.keys.updating = true
	if snapshot.layer == 1 {
		u.keys.layer.SetSelected(u.i18n.T("layer.fn"))
	} else {
		u.keys.layer.SetSelected(u.i18n.T("layer.base"))
	}
	u.keys.updating = false
	u.setKeyMapping(&u.keys, snapshot.mapping)
	u.setLightingSettings(&u.lighting, snapshot.lighting)
	u.setActuationSettings(&u.actuation, snapshot.actuation)
	u.setMPTStages(snapshot.mpt)
	u.macros.actions = snapshot.macros
	u.macros.selected = -1
	u.macros.list.UnselectAll()
	u.macros.list.Refresh()
}

func (u *UI) setMPTStages(stages []protocol.MPTStage) {
	t := u.i18n.T
	for i, value := range stages {
		if i >= len(u.mpt.stages) {
			break
		}
		if value.PressMM > 0 {
			u.mpt.stages[i].press.SetValue(value.PressMM)
		}
		if value.ReleaseMM > 0 {
			u.mpt.stages[i].release.SetValue(value.ReleaseMM)
		}
		if value.Output == 0 {
			u.mpt.stages[i].output.SetSelected(t("output.disabled"))
		} else {
			u.mpt.stages[i].output.SetSelected(protocol.KeyName(value.Output))
		}
	}
}

func (u *UI) startAutoDiscovery() {
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-u.done:
				return
			case <-ticker.C:
				fyne.Do(func() {
					if u.autoConnect && u.client == nil && !u.busy {
						u.refreshDevices()
					}
				})
			}
		}
	}()
}

func (u *UI) changeProfile(value string) {
	if u.updating || u.client == nil {
		return
	}
	profile := 0
	if value == u.i18n.T("profile.2") {
		profile = 1
	}
	u.run(func(ctx context.Context) (func(), error) {
		if err := u.client.SetProfile(ctx, profile); err != nil {
			return nil, err
		}
		configuration, err := u.readConfiguration(ctx, u.client)
		if err != nil {
			return nil, err
		}
		return func() {
			u.applyConfiguration(configuration)
			u.detail.SetText(u.i18n.T("status.profile"))
		}, nil
	})
}

func (u *UI) clearLoadedState() {
	u.keys.mapping = nil
	u.keys.current.SetText("—")
	u.updateKeyKeyboard(&u.keys)
	u.actuation.settings = nil
	u.actuation.mode.SetText("—")
	u.updateActuationKeyboard(&u.actuation)
	u.macros.actions = nil
	u.macros.list.Refresh()
}

func (u *UI) run(work func(context.Context) (func(), error)) {
	if u.client == nil {
		dialog.ShowInformation(u.i18n.T("action.connect"), u.i18n.T("error.no_device"), u.window)
		return
	}
	if u.busy {
		return
	}
	u.setBusy(true, u.i18n.T("connection.working"))
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		apply, err := work(ctx)
		fyne.Do(func() { u.finish(apply, err) })
	}()
}

func (u *UI) finish(apply func(), err error) {
	u.setBusy(false, "")
	if err != nil {
		u.showError(err)
		return
	}
	if apply != nil {
		apply()
	}
}

func (u *UI) setBusy(busy bool, message string) {
	u.busy = busy
	if busy {
		u.connectButton.Disable()
		u.deviceSelect.Disable()
		u.profileSelect.Disable()
		u.status.SetText(message)
		return
	}
	u.connectButton.Enable()
	if u.client == nil {
		u.deviceSelect.Enable()
		u.profileSelect.Disable()
		u.status.SetText(u.i18n.T("connection.disconnected"))
	} else {
		u.deviceSelect.Disable()
		u.profileSelect.Enable()
		u.status.SetText(u.i18n.T("connection.ready"))
	}
}

func (u *UI) showError(err error) {
	dialog.ShowError(fmt.Errorf("%s: %w", u.i18n.T("error.communication"), err), u.window)
}

func (u *UI) confirmReset(feature string, confirmed func()) {
	dialog.ShowConfirm(
		u.i18n.T("action.reset"),
		fmt.Sprintf(u.i18n.T("reset.confirm"), feature),
		func(ok bool) {
			if ok {
				confirmed()
			}
		},
		u.window,
	)
}

func (u *UI) changeTheme(value string) {
	setting := "system"
	switch value {
	case u.i18n.T("theme.light"):
		setting = "light"
		u.app.Settings().SetTheme(theme.LightTheme())
	case u.i18n.T("theme.dark"):
		setting = "dark"
		u.app.Settings().SetTheme(theme.DarkTheme())
	default:
		u.app.Settings().SetTheme(theme.DefaultTheme())
	}
	u.app.Preferences().SetString(preferenceTheme, setting)
	u.updateKeyKeyboard(&u.keys)
	u.updateActuationKeyboard(&u.actuation)
}

func (u *UI) applySavedTheme() {
	switch u.app.Preferences().StringWithFallback(preferenceTheme, "system") {
	case "light":
		u.app.Settings().SetTheme(theme.LightTheme())
	case "dark":
		u.app.Settings().SetTheme(theme.DarkTheme())
	default:
		u.app.Settings().SetTheme(theme.DefaultTheme())
	}
}

func (u *UI) themeLabel(setting string) string {
	switch setting {
	case "light":
		return u.i18n.T("theme.light")
	case "dark":
		return u.i18n.T("theme.dark")
	default:
		return u.i18n.T("theme.system")
	}
}

func rgbaBytes(value color.Color) (byte, byte, byte) {
	r, g, b, _ := value.RGBA()
	return byte(r >> 8), byte(g >> 8), byte(b >> 8)
}
