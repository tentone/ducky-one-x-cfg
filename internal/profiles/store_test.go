package profiles

import (
	"path/filepath"
	"testing"

	"github.com/joseferrao/ducky-drv/internal/protocol"
)

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
