package ui

import (
	"errors"
	"fmt"
	"runtime"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/tentone/ducky-drv/internal/profiles"
	"github.com/tentone/ducky-drv/internal/shortcuts"
)

type profileShortcutManager interface {
	Set(string, string) error
	Close()
}

func displayProfileShortcut(value string) string {
	if runtime.GOOS == "windows" {
		return strings.ReplaceAll(value, "Super", "Win")
	}
	if runtime.GOOS == "darwin" {
		return strings.ReplaceAll(value, "Super", "Cmd")
	}
	return value
}

func (u *UI) initializeProfileShortcuts() {
	u.profileShortcuts = shortcuts.New(func(id string) {
		fyne.Do(func() { u.dispatchProfileShortcut(id) })
	})
	for _, profile := range u.profileStore.Profiles() {
		if err := u.profileShortcuts.Set(profile.ID, profile.Shortcut); err != nil {
			u.notifyProfileShortcut(fmt.Sprintf(u.i18n.T("shortcut.unavailable"), profile.Name, err))
		}
	}
}

func (u *UI) dispatchProfileShortcut(id string) {
	select {
	case <-u.done:
		return
	default:
	}
	if u.profileEditor != nil || u.profileStore == nil {
		return
	}
	if profile, ok := u.profileStore.Get(id); ok {
		u.loadSoftwareProfileWithFeedback(profile, true)
	}
}

func (u *UI) notifyProfileShortcut(message string) {
	u.app.SendNotification(fyne.NewNotification(u.i18n.T("app.title"), message))
}

func (u *UI) saveSoftwareProfileMetadata(profile profiles.Profile, name, value string) (profiles.Profile, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return profiles.Profile{}, errors.New(u.i18n.T("software_profiles.name_required"))
	}
	shortcut, err := shortcuts.Parse(value)
	if err != nil {
		return profiles.Profile{}, err
	}
	value = shortcut.String()
	for _, other := range u.profileStore.Profiles() {
		if other.ID != profile.ID && value != "" && other.Shortcut == value {
			return profiles.Profile{}, fmt.Errorf(u.i18n.T("shortcut.duplicate"), other.Name)
		}
	}
	if u.profileShortcuts != nil {
		if err := u.profileShortcuts.Set(profile.ID, value); err != nil {
			return profiles.Profile{}, fmt.Errorf(u.i18n.T("shortcut.unavailable"), name, err)
		}
	}
	saved, err := u.profileStore.Edit(profile.ID, name, value)
	if err != nil && u.profileShortcuts != nil {
		err = errors.Join(err, u.profileShortcuts.Set(profile.ID, profile.Shortcut))
	}
	return saved, err
}

func (u *UI) openEditSoftwareProfile(id string) {
	if u.busy || u.profileEditor != nil || u.profileStore == nil {
		return
	}
	profile, ok := u.profileStore.Get(id)
	if !ok {
		return
	}
	t := u.i18n.T
	name := widget.NewEntry()
	name.SetText(profile.Name)
	ctrl, alt, shift := widget.NewCheck("Ctrl", nil), widget.NewCheck("Alt", nil), widget.NewCheck("Shift", nil)
	superName := "Super"
	if runtime.GOOS == "windows" {
		superName = "Win"
	} else if runtime.GOOS == "darwin" {
		superName = "Cmd"
	}
	super := widget.NewCheck(superName, nil)
	key := widget.NewSelect(shortcuts.Keys(), nil)
	key.PlaceHolder = t("shortcut.key")
	current, _ := shortcuts.Parse(profile.Shortcut)
	ctrl.Checked, alt.Checked, shift.Checked, super.Checked, key.Selected = current.Ctrl, current.Alt, current.Shift, current.Super, current.Key
	preview := widget.NewLabel("")
	update := func() {
		value := (shortcuts.Shortcut{Ctrl: ctrl.Checked, Alt: alt.Checked, Shift: shift.Checked, Super: super.Checked, Key: key.Selected}).String()
		if value == "" {
			value = t("shortcut.none")
		}
		preview.SetText(displayProfileShortcut(value))
	}
	for _, check := range []*widget.Check{ctrl, alt, shift, super} {
		check.OnChanged = func(bool) { update() }
	}
	key.OnChanged = func(string) { update() }
	update()
	clear := widget.NewButton(t("shortcut.clear"), func() {
		ctrl.SetChecked(false)
		alt.SetChecked(false)
		shift.SetChecked(false)
		super.SetChecked(false)
		key.ClearSelected()
	})
	shortcutControls := container.NewVBox(container.NewHBox(ctrl, alt, shift, super), container.NewBorder(nil, nil, nil, clear, key), preview)
	form := widget.NewForm(widget.NewFormItem(t("software_profiles.name"), name), widget.NewFormItem(t("shortcut.label"), shortcutControls))
	hint := widget.NewLabel(displayProfileShortcut(t("shortcut.hint")))
	hint.Wrapping = fyne.TextWrapWord
	problem := widget.NewLabel("")
	problem.Wrapping = fyne.TextWrapWord
	problem.Importance = widget.DangerImportance
	problem.Hide()
	save := widget.NewButton(t("software_profiles.save"), func() {
		if u.busy {
			problem.SetText(t("shortcut.busy"))
			problem.Show()
			return
		}
		value := (shortcuts.Shortcut{Ctrl: ctrl.Checked, Alt: alt.Checked, Shift: shift.Checked, Super: super.Checked, Key: key.Selected}).String()
		if key.Selected == "" && (ctrl.Checked || alt.Checked || shift.Checked || super.Checked) {
			problem.SetText(t("shortcut.key"))
			problem.Show()
			return
		}
		saved, err := u.saveSoftwareProfileMetadata(profile, name.Text, value)
		if err != nil {
			problem.SetText(err.Error())
			problem.Show()
			return
		}
		u.profileEditor.Hide()
		u.refreshSoftwareProfiles(saved.ID)
	})
	save.Importance = widget.HighImportance
	modal := dialog.NewCustomWithoutButtons(t("software_profiles.edit"), container.NewVBox(form, hint, problem), u.window)
	modal.SetButtons([]fyne.CanvasObject{widget.NewButton(t("action.cancel"), modal.Hide), save})
	u.profileEditor = modal
	u.profileEditor.SetOnClosed(func() { u.profileEditor = nil })
	u.profileEditor.Resize(fyne.NewSize(540, 340))
	u.profileEditor.Show()
}

func (u *UI) deleteSoftwareProfile(profile profiles.Profile) error {
	if u.profileShortcuts != nil {
		if err := u.profileShortcuts.Set(profile.ID, ""); err != nil {
			return err
		}
	}
	err := u.profileStore.Delete(profile.ID)
	if err != nil && u.profileShortcuts != nil {
		err = errors.Join(err, u.profileShortcuts.Set(profile.ID, profile.Shortcut))
	}
	return err
}
