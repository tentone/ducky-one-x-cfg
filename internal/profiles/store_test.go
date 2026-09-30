package profiles

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/tentone/ducky-drv/internal/protocol"
)

func TestMetadataEditPersistsShortcutAndRejectsDuplicatesAtomically(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profiles.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	one, err := s.Create("Original", validConfiguration())
	if err != nil {
		t.Fatal(err)
	}
	two, err := s.Create("Other", validConfiguration())
	if err != nil {
		t.Fatal(err)
	}
	edited, err := s.Edit(one.ID, " Renamed ", "alt + ctrl + 1")
	if err != nil || edited.Name != "Renamed" || edited.Shortcut != "Ctrl+Alt+1" {
		t.Fatalf("edit=%+v, %v", edited, err)
	}
	if !reflect.DeepEqual(edited.Configuration, one.Configuration) {
		t.Fatal("metadata edit changed the keyboard snapshot")
	}
	if _, err := s.Edit(two.ID, "Should not save", "Ctrl+Alt+1"); err == nil {
		t.Fatal("duplicate shortcut accepted")
	}
	if _, err := s.Edit(one.ID, "Should not save", "Shift+A"); err == nil {
		t.Fatal("invalid shortcut accepted")
	}
	reloaded, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := reloaded.Get(one.ID)
	other, _ := reloaded.Get(two.ID)
	if got.Name != "Renamed" || got.Shortcut != "Ctrl+Alt+1" || other.Name != "Other" || other.Shortcut != "" {
		t.Fatal("shortcut did not persist or failed edits changed metadata")
	}
	if _, err := s.Edit(one.ID, "Renamed", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Edit(two.ID, "Other", "Ctrl+Alt+1"); err != nil {
		t.Fatalf("cleared shortcut unavailable: %v", err)
	}
}

func TestStoreRoundTripAndUnlimitedProfiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "profiles.json")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	configuration := validConfiguration()
	for index := 0; index < 40; index++ {
		if _, err := store.Create("Profile "+string(rune('A'+index)), configuration); err != nil {
			t.Fatalf("create profile %d: %v", index, err)
		}
	}
	if got := len(store.Profiles()); got != 40 {
		t.Fatalf("profile count = %d, want 40", got)
	}

	reloaded, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	profiles := reloaded.Profiles()
	if len(profiles) != 40 || len(profiles[0].Configuration.KeyMaps[1]) != protocol.KeyMapEntries {
		t.Fatal("saved profiles did not round-trip")
	}

	profiles[0].Configuration.KeyMaps[0][0].Code = 99
	stored, _ := reloaded.Get(profiles[0].ID)
	if stored.Configuration.KeyMaps[0][0].Code == 99 {
		t.Fatal("returned profiles share mutable configuration storage")
	}
}

func TestStoreUpdateRenameDelete(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "profiles.json"))
	if err != nil {
		t.Fatal(err)
	}
	profile, err := store.Create("Original", validConfiguration())
	if err != nil {
		t.Fatal(err)
	}
	configuration := validConfiguration()
	configuration.Lighting.Brightness = 75
	updated, err := store.Update(profile.ID, configuration)
	if err != nil || updated.Configuration.Lighting.Brightness != 75 {
		t.Fatalf("update = %+v, %v", updated, err)
	}
	renamed, err := store.Rename(profile.ID, "Renamed")
	if err != nil || renamed.Name != "Renamed" {
		t.Fatalf("rename = %+v, %v", renamed, err)
	}
	if err := store.Delete(profile.ID); err != nil {
		t.Fatal(err)
	}
	if len(store.Profiles()) != 0 {
		t.Fatal("deleted profile remains in store")
	}
	if _, found := store.Get(profile.ID); found {
		t.Fatal("deleted profile can still be retrieved")
	}
}

func TestValidateRejectsIncompleteProfile(t *testing.T) {
	configuration := validConfiguration()
	configuration.Macros = configuration.Macros[:13]
	if err := Validate(configuration); err == nil {
		t.Fatal("incomplete macro set was accepted")
	}
}

func validConfiguration() Configuration {
	configuration := Configuration{
		KeyMaps:   make([][]protocol.Assignment, 2),
		Actuation: make([]protocol.ActuationSetting, protocol.MaxMatrixKeys),
		MPT:       make([][]protocol.MPTStage, protocol.MPTPresets),
		Macros:    make([][]protocol.MacroAction, protocol.MacroSlots),
	}
	for layer := range configuration.KeyMaps {
		configuration.KeyMaps[layer] = make([]protocol.Assignment, protocol.KeyMapEntries)
	}
	for preset := range configuration.MPT {
		configuration.MPT[preset] = make([]protocol.MPTStage, 4)
	}
	return configuration
}
