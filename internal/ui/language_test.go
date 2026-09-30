package ui

import (
	"testing"

	fyneTest "fyne.io/fyne/v2/test"
	"github.com/joseferrao/ducky-drv/internal/i18n"
	"github.com/joseferrao/ducky-drv/internal/protocol"
)

func TestLanguageNativeNameMapping(t *testing.T) {
	for _, want := range i18n.Languages() {
		got, ok := languageFromNativeName(i18n.NativeName(want))
		if !ok || got != want {
			t.Fatalf("languageFromNativeName(%q) = %q, %v; want %q, true", i18n.NativeName(want), got, ok, want)
		}
	}
	if _, ok := languageFromNativeName("not installed"); ok {
		t.Fatal("unknown language name should not resolve")
	}
}

func TestLanguageSwitchRebuildsUIAndPreservesEdits(t *testing.T) {
	application := fyneTest.NewApp()
	defer application.Quit()
	u := &UI{
		app: application, window: application.NewWindow("test"),
		i18n: i18n.New(), done: make(chan struct{}),
	}
	u.build()
	u.window.SetContent(u.content())
	defer func() { u.lighting.animator.Stop() }()

	u.keys.mapping = []protocol.Assignment{{Kind: protocol.AssignmentKeyboard, Code: 4}}
	u.lighting.effect.SetSelected(effectName(21, u.i18n.T))
	u.lighting.brightness.SetValue(75)
	u.macros.actions = []protocol.MacroAction{{Kind: protocol.MacroText, Text: "test"}}
	u.openSettings()

	u.changeLanguage(i18n.NativeName(i18n.French))
	if u.settingsDialog == nil || u.languageSelect.Selected != i18n.NativeName(i18n.French) {
		t.Fatal("settings should remain open in the selected language")
	}
	u.settingsDialog.Hide()
	if u.i18n.Language() != i18n.French {
		t.Fatalf("active language = %q, want French", u.i18n.Language())
	}
	if u.keys.layer.Selected != "Couche de base" {
		t.Fatalf("translated base layer = %q", u.keys.layer.Selected)
	}
	if u.lighting.effect.Selected != "Arc-en-ciel" || u.lighting.brightness.Value != 75 {
		t.Fatalf("lighting edit was not preserved: effect=%q brightness=%v", u.lighting.effect.Selected, u.lighting.brightness.Value)
	}
	if len(u.keys.mapping) != 1 || len(u.macros.actions) != 1 || u.macros.actions[0].Text != "test" {
		t.Fatal("configuration edits were not preserved across the language switch")
	}
	if got := application.Preferences().String(preferenceLanguage); got != string(i18n.French) {
		t.Fatalf("saved language = %q, want %q", got, i18n.French)
	}
}
