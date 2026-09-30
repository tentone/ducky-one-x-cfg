package ui

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	fyneTest "fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/tentone/ducky-drv/internal/i18n"
	"github.com/tentone/ducky-drv/internal/profiles"
	"github.com/tentone/ducky-drv/internal/protocol"
)

type shortcutRecorder struct {
	values []string
	fail   bool
}

func TestClickProfileOpensEditorAndSavesNameAndShortcut(t *testing.T) {
	app := fyneTest.NewApp()
	defer app.Quit()
	store, err := profiles.Open(filepath.Join(t.TempDir(), "profiles.json"))
	if err != nil {
		t.Fatal(err)
	}
	profile, err := store.Create("Gaming", completeTestConfiguration())
	if err != nil {
		t.Fatal(err)
	}
	backend := &shortcutRecorder{}
	u := &UI{app: app, window: app.NewWindow("test"), i18n: i18n.New(), profileStore: store, profileShortcuts: backend, done: make(chan struct{})}
	u.build()
	defer u.lighting.animator.Stop()
	u.window.SetContent(u.content())
	row := u.softwareProfiles.list.CreateItem()
	u.softwareProfiles.list.UpdateItem(0, row)
	fyneTest.Tap(row.(*widget.Button))
	if u.profileEditor == nil {
		t.Fatal("clicking profile did not open editor")
	}
	var name *widget.Entry
	var key *widget.Select
	var save *widget.Button
	checks := map[string]*widget.Check{}
	var visit func(fyne.CanvasObject)
	visit = func(object fyne.CanvasObject) {
		switch object := object.(type) {
		case *fyne.Container:
			for _, child := range object.Objects {
				visit(child)
			}
		case *widget.Form:
			for _, item := range object.Items {
				visit(item.Widget)
			}
		case *widget.Entry:
			name = object
		case *widget.Select:
			key = object
		case *widget.Check:
			checks[object.Text] = object
		case *widget.Button:
			if object.Text == u.i18n.T("software_profiles.save") {
				save = object
			}
		case *widget.PopUp:
			visit(object.Content)
		case fyne.Widget:
			for _, child := range fyneTest.WidgetRenderer(object).Objects() {
				visit(child)
			}
		}
	}
	for _, overlay := range u.window.Canvas().Overlays().List() {
		visit(overlay)
	}
	if name == nil || key == nil || save == nil || checks["Ctrl"] == nil || checks["Alt"] == nil {
		t.Fatal("profile editor controls missing")
	}
	name.SetText("Office")
	checks["Ctrl"].SetChecked(true)
	checks["Alt"].SetChecked(true)
	key.SetSelected("2")
	fyneTest.Tap(save)
	if u.profileEditor != nil {
		t.Fatal("successful save did not close editor")
	}
	saved, _ := store.Get(profile.ID)
	if saved.Name != "Office" || saved.Shortcut != "Ctrl+Alt+2" || len(backend.values) != 1 || backend.values[0] != saved.Shortcut {
		t.Fatalf("editor did not save and register shortcut: %+v, %q", saved, backend.values)
	}
}

func (r *shortcutRecorder) Set(_ string, value string) error {
	if r.fail {
		return errors.New("shortcut claimed by another app")
	}
	r.values = append(r.values, value)
	return nil
}
func (*shortcutRecorder) Close() {}

func TestProfileEditPreservesMetadataOnShortcutOrDiskFailure(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "library")
	store, err := profiles.Open(filepath.Join(parent, "profiles.json"))
	if err != nil {
		t.Fatal(err)
	}
	profile, err := store.Create("Original", completeTestConfiguration())
	if err != nil {
		t.Fatal(err)
	}
	profile, err = store.Edit(profile.ID, profile.Name, "Ctrl+Alt+1")
	if err != nil {
		t.Fatal(err)
	}
	backend := &shortcutRecorder{fail: true}
	u := &UI{i18n: i18n.New(), profileStore: store, profileShortcuts: backend}
	if _, err := u.saveSoftwareProfileMetadata(profile, "Changed", "Ctrl+Alt+2"); err == nil {
		t.Fatal("registration failure accepted")
	}
	got, _ := store.Get(profile.ID)
	if got.Name != "Original" || got.Shortcut != "Ctrl+Alt+1" {
		t.Fatal("failed registration changed the profile")
	}
	backend.fail = false
	// Make the store's parent unusable without altering anything outside this test.
	if err := os.Remove(store.Path()); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(parent); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(parent, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := u.saveSoftwareProfileMetadata(profile, "Changed", "Ctrl+Alt+2"); err == nil {
		t.Fatal("persistence failure accepted")
	}
	if len(backend.values) != 2 || backend.values[0] != "Ctrl+Alt+2" || backend.values[1] != "Ctrl+Alt+1" {
		t.Fatalf("old shortcut was not restored: %q", backend.values)
	}
	got, _ = store.Get(profile.ID)
	if got.Name != "Original" || got.Shortcut != "Ctrl+Alt+1" {
		t.Fatal("failed save changed the in-memory profile")
	}
}

type notifyingApp struct {
	fyne.App
	notifications chan *fyne.Notification
}

func (a *notifyingApp) SendNotification(n *fyne.Notification) {
	a.App.SendNotification(n)
	a.notifications <- n
}

func TestShortcutLoadsCompleteProfileWithoutOpeningDialog(t *testing.T) {
	app := fyneTest.NewApp()
	defer app.Quit()
	notifications := make(chan *fyne.Notification, 4)
	store, err := profiles.Open(filepath.Join(t.TempDir(), "profiles.json"))
	if err != nil {
		t.Fatal(err)
	}
	profile, err := store.Create("Gaming", completeTestConfiguration())
	if err != nil {
		t.Fatal(err)
	}
	recorder := &profileWriteRecorder{commands: make(map[byte]int)}
	u := &UI{app: &notifyingApp{App: app, notifications: notifications}, window: app.NewWindow("test"), i18n: i18n.New(), profileStore: store, done: make(chan struct{})}
	u.build()
	defer u.lighting.animator.Stop()
	u.window.SetContent(u.content())
	u.client = protocol.NewClient(recorder)
	u.profileSelect.Selected = u.i18n.T("profile.1")
	u.window.Hide()
	u.dispatchProfileShortcut(profile.ID)
	select {
	case n := <-notifications:
		if n.Content != "Software profile \"Gaming\" loaded into Profile 1." {
			t.Fatalf("notification: %q", n.Content)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("background load did not complete")
	}
	if recorder.commands[27] != 10 || recorder.commands[31] != protocol.MacroSlots*8 {
		t.Fatalf("shortcut did not write the complete profile: %v", recorder.commands)
	}
	if len(u.window.Canvas().Overlays().List()) != 0 {
		t.Fatal("shortcut opened a dialog")
	}
	u.client = nil
	u.dispatchProfileShortcut(profile.ID)
	select {
	case n := <-notifications:
		if n.Content != u.i18n.T("error.no_device") {
			t.Fatal(n.Content)
		}
	default:
		t.Fatal("missing device did not notify")
	}
	if len(u.window.Canvas().Overlays().List()) != 0 {
		t.Fatal("missing device opened a dialog")
	}
}
