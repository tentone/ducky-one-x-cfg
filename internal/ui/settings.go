package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/tentone/ducky-drv/internal/i18n"
	"github.com/tentone/ducky-drv/internal/startup"
)

func (u *UI) openSettings() {
	if u.settingsDialog != nil {
		return
	}
	t := u.i18n.T
	themeSelect := widget.NewSelect([]string{t("theme.system"), t("theme.light"), t("theme.dark")}, u.changeTheme)
	themeSelect.Selected = u.themeLabel(u.app.Preferences().StringWithFallback(preferenceTheme, "system"))
	languages := i18n.Languages()
	names := make([]string, len(languages))
	for index, language := range languages {
		names[index] = i18n.NativeName(language)
	}
	u.languageSelect = widget.NewSelect(names, u.changeLanguage)
	u.languageSelect.Selected = i18n.NativeName(u.i18n.Language())
	launch := widget.NewCheck(t("settings.startup"), nil)
	launch.Checked = u.app.Preferences().BoolWithFallback(preferenceStartup, startup.Enabled())
	launch.OnChanged = func(enabled bool) {
		if err := startup.SetEnabled(enabled); err != nil {
			launch.Checked = !enabled
			launch.Refresh()
			u.showError(err)
			return
		}
		u.app.Preferences().SetBool(preferenceStartup, enabled)
	}
	form := widget.NewForm(widget.NewFormItem(t("theme"), themeSelect), widget.NewFormItem(t("language"), u.languageSelect))
	hint := widget.NewLabel(t("settings.startup_hint"))
	hint.Wrapping = fyne.TextWrapWord
	content := container.NewVBox(form, widget.NewSeparator(), launch, hint)
	u.settingsDialog = dialog.NewCustom(t("settings.title"), t("action.close"), content, u.window)
	u.settingsDialog.SetOnClosed(func() { u.settingsDialog = nil })
	u.settingsDialog.Resize(fyne.NewSize(480, 260))
	u.settingsDialog.Show()
}
