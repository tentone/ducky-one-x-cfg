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
		i18n: i18n.New(), manager: manager,
	}
	u.window.SetOnClosed(func() {
		if err := manager.Close(); err != nil {
			log.Printf("close HID manager: %v", err)
		}
	})
	u.build()
	u.applySavedTheme()
	u.window.Resize(fyne.NewSize(1120, 760))
	u.window.SetContent(u.content())
	u.refreshDevices()
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
	subtitle := widget.NewLabel(t("app.subtitle"))
	subtitle.Importance = widget.LowImportance

	refresh := widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), u.refreshDevices)
	refresh.Importance = widget.LowImportance
	themeSelect := widget.NewSelect(
		[]string{t("theme.system"), t("theme.light"), t("theme.dark")},
		u.changeTheme,
	)
	themeSelect.SetSelected(u.themeLabel(u.app.Preferences().StringWithFallback(preferenceTheme, "system")))
	language := widget.NewSelect([]string{"English"}, func(string) {})
	language.SetSelected("English")

	header := container.NewBorder(
		nil, nil,
		container.NewVBox(title, subtitle),
		container.NewHBox(
			widget.NewLabel(t("profile")), u.profileSelect,
			widget.NewLabel(t("theme")), themeSelect,
			widget.NewLabel(t("language")), language,
		),
		container.NewBorder(nil, nil, nil, container.NewHBox(refresh, u.connectButton), u.deviceSelect),
	)

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
	} else {
		u.deviceSelect.ClearSelected()
		u.detail.SetText(u.i18n.T("connection.wired"))
	}
}

func (u *UI) toggleConnection() {
	if u.client != nil {
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
	selected := u.deviceSelect.Selected
	index := -1
	for i, descriptor := range u.devices {
		if descriptor.DisplayName() == selected {
			index = i
			break
		}
	}
	if index < 0 {
		u.refreshDevices()
		if len(u.devices) == 0 {
			dialog.ShowInformation(u.i18n.T("action.connect"), u.i18n.T("connection.wired"), u.window)
			return
		}
		index = 0
	}
	descriptor := u.devices[index]
	u.setBusy(true, fmt.Sprintf(u.i18n.T("connection.opening"), descriptor.DisplayName()))
	go func() {
		session, err := u.manager.Connect(descriptor.Path)
		if err != nil {
			fyne.Do(func() { u.finish(nil, err) })
			return
		}
		client := protocol.NewClient(session)
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		defer cancel()
		metadata, metadataErr := client.Metadata(ctx)
		profile, profileErr := client.ActiveProfile(ctx)
		fyne.Do(func() {
			if metadataErr != nil {
				_ = u.manager.Disconnect()
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
			u.status.SetText(u.i18n.T("connection.ready"))
			u.detail.SetText(fmt.Sprintf(u.i18n.T("device.summary"), descriptor.DisplayName(), metadata.Firmware()))
			u.finish(nil, nil)
		})
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
		return func() {
			u.clearLoadedState()
			u.detail.SetText(u.i18n.T("status.profile"))
		}, nil
	})
}

func (u *UI) clearLoadedState() {
	u.keys.mapping = nil
	u.keys.current.SetText("—")
	u.actuation.settings = nil
	u.actuation.mode.SetText("—")
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
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
