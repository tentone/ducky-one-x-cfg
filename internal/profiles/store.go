// Package profiles persists software-managed keyboard profiles.
package profiles

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/joseferrao/ducky-drv/internal/protocol"
	shortcuts "github.com/joseferrao/ducky-drv/internal/shortcuts/spec"
)

const fileVersion = 1

// Configuration is a complete snapshot of one onboard memory profile.
type Configuration struct {
	ActiveLayer    int                             `json:"active_layer"`
	KeyMaps        [][]protocol.Assignment         `json:"key_maps"`
	Lighting       protocol.LightingSettings       `json:"lighting"`
	CustomLighting protocol.CustomLightingSettings `json:"custom_lighting"`
	Actuation      []protocol.ActuationSetting     `json:"actuation"`
	MPT            [][]protocol.MPTStage           `json:"mpt"`
	Macros         [][]protocol.MacroAction        `json:"macros"`
}

// Profile is a named software profile with stable identity and timestamps.
type Profile struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	Shortcut      string        `json:"shortcut,omitempty"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
	Configuration Configuration `json:"configuration"`
}

type profileFile struct {
	Version  int       `json:"version"`
	Profiles []Profile `json:"profiles"`
}

// Store owns a JSON profile library at Path().
type Store struct {
	mu       sync.RWMutex
	path     string
	profiles []Profile
}

// DefaultPath returns the platform-appropriate profile library path.
func DefaultPath() (string, error) {
	directory, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("find user configuration directory: %w", err)
	}
	return filepath.Join(directory, "ducky-one-x-configurator", "profiles.json"), nil
}

// Open loads a profile store. A missing file creates an empty in-memory store.
func Open(path string) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("profile store path is empty")
	}
	store := &Store{path: path}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return store, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read software profiles: %w", err)
	}
	var file profileFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("decode software profiles: %w", err)
	}
	if file.Version != fileVersion {
		return nil, fmt.Errorf("unsupported software profile file version %d", file.Version)
	}
	seen := make(map[string]bool, len(file.Profiles))
	seenShortcuts := make(map[string]bool)
	for index := range file.Profiles {
		profile := &file.Profiles[index]
		profile.Name = strings.TrimSpace(profile.Name)
		if profile.ID == "" || seen[profile.ID] {
			return nil, fmt.Errorf("software profile %d has an invalid ID", index+1)
		}
		seen[profile.ID] = true
		shortcut, err := shortcuts.Parse(profile.Shortcut)
		if err != nil {
			return nil, fmt.Errorf("software profile %q shortcut: %w", profile.Name, err)
		}
		profile.Shortcut = shortcut.String()
		if profile.Shortcut != "" {
			if seenShortcuts[profile.Shortcut] {
				return nil, fmt.Errorf("duplicate profile shortcut %q", profile.Shortcut)
			}
			seenShortcuts[profile.Shortcut] = true
		}
		if profile.Name == "" {
			return nil, fmt.Errorf("software profile %d has no name", index+1)
		}
		if err := Validate(profile.Configuration); err != nil {
			return nil, fmt.Errorf("software profile %q: %w", profile.Name, err)
		}
	}
	store.profiles = cloneProfiles(file.Profiles)
	return store, nil
}

// OpenDefault opens the profile library in the operating system's user config directory.
func OpenDefault() (*Store, error) {
	path, err := DefaultPath()
	if err != nil {
		return nil, err
	}
	return Open(path)
}

// Path returns the JSON file used by this store.
func (s *Store) Path() string { return s.path }

// Profiles returns independent copies in display order.
func (s *Store) Profiles() []Profile {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneProfiles(s.profiles)
}

// Get returns a profile by stable ID.
func (s *Store) Get(id string) (Profile, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, profile := range s.profiles {
		if profile.ID == id {
			return cloneProfile(profile), true
		}
	}
	return Profile{}, false
}

// Create adds and persists a new profile. The number of profiles is not limited.
func (s *Store) Create(name string, configuration Configuration) (Profile, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Profile{}, errors.New("profile name is empty")
	}
	if err := Validate(configuration); err != nil {
		return Profile{}, err
	}
	id, err := newID()
	if err != nil {
		return Profile{}, err
	}
	now := time.Now().UTC()
	profile := Profile{ID: id, Name: name, CreatedAt: now, UpdatedAt: now, Configuration: cloneConfiguration(configuration)}

	s.mu.Lock()
	defer s.mu.Unlock()
	next := append(cloneProfiles(s.profiles), profile)
	if err := s.persist(next); err != nil {
		return Profile{}, err
	}
	s.profiles = next
	return cloneProfile(profile), nil
}

// Update replaces a profile's configuration with a fresh keyboard snapshot.
func (s *Store) Update(id string, configuration Configuration) (Profile, error) {
	if err := Validate(configuration); err != nil {
		return Profile{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := cloneProfiles(s.profiles)
	for index := range next {
		if next[index].ID != id {
			continue
		}
		next[index].Configuration = cloneConfiguration(configuration)
		next[index].UpdatedAt = time.Now().UTC()
		if err := s.persist(next); err != nil {
			return Profile{}, err
		}
		s.profiles = next
		return cloneProfile(next[index]), nil
	}
	return Profile{}, errors.New("software profile not found")
}

// Rename changes a profile's display name.
func (s *Store) Rename(id, name string) (Profile, error) {
	return s.editMetadata(id, name, nil)
}

// Edit atomically saves profile metadata without changing its configuration.
func (s *Store) Edit(id, name, shortcut string) (Profile, error) {
	parsed, err := shortcuts.Parse(shortcut)
	if err != nil {
		return Profile{}, err
	}
	shortcut = parsed.String()
	return s.editMetadata(id, name, &shortcut)
}

func (s *Store) editMetadata(id, name string, shortcut *string) (Profile, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Profile{}, errors.New("profile name is empty")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := cloneProfiles(s.profiles)
	for _, profile := range next {
		if profile.ID != id && shortcut != nil && *shortcut != "" && profile.Shortcut == *shortcut {
			return Profile{}, errors.New("shortcut already assigned to another profile")
		}
	}
	for index := range next {
		if next[index].ID != id {
			continue
		}
		next[index].Name = name
		if shortcut != nil {
			next[index].Shortcut = *shortcut
		}
		next[index].UpdatedAt = time.Now().UTC()
		if err := s.persist(next); err != nil {
			return Profile{}, err
		}
		s.profiles = next
		return cloneProfile(next[index]), nil
	}
	return Profile{}, errors.New("software profile not found")
}

// Delete removes a profile from the local library.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := make([]Profile, 0, len(s.profiles))
	found := false
	for _, profile := range s.profiles {
		if profile.ID == id {
			found = true
			continue
		}
		next = append(next, cloneProfile(profile))
	}
	if !found {
		return errors.New("software profile not found")
	}
	if err := s.persist(next); err != nil {
		return err
	}
	s.profiles = next
	return nil
}

// Validate checks that a snapshot contains every configurable onboard item.
func Validate(configuration Configuration) error {
	if configuration.ActiveLayer < 0 || configuration.ActiveLayer > 1 {
		return errors.New("active layer must be 0 or 1")
	}
	if len(configuration.KeyMaps) != 2 {
		return errors.New("profile must contain both key layers")
	}
	for layer, mapping := range configuration.KeyMaps {
		if len(mapping) != protocol.KeyMapEntries {
			return fmt.Errorf("key layer %d contains %d entries, expected %d", layer, len(mapping), protocol.KeyMapEntries)
		}
	}
	if len(configuration.Actuation) != protocol.MaxMatrixKeys {
		return fmt.Errorf("actuation contains %d entries, expected %d", len(configuration.Actuation), protocol.MaxMatrixKeys)
	}
	if len(configuration.MPT) != protocol.MPTPresets {
		return fmt.Errorf("profile contains %d MPT presets, expected %d", len(configuration.MPT), protocol.MPTPresets)
	}
	for preset, stages := range configuration.MPT {
		if len(stages) != 4 {
			return fmt.Errorf("MPT%d contains %d stages, expected 4", preset+1, len(stages))
		}
	}
	if len(configuration.Macros) != protocol.MacroSlots {
		return fmt.Errorf("profile contains %d macro slots, expected %d", len(configuration.Macros), protocol.MacroSlots)
	}
	for _, keyColor := range configuration.CustomLighting.Colors {
		if int(keyColor.Key) >= protocol.MaxMatrixKeys {
			return fmt.Errorf("custom lighting key %d is outside the keyboard matrix", keyColor.Key)
		}
	}
	return nil
}

func (s *Store) persist(profiles []Profile) error {
	directory := filepath.Dir(s.path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create software profile directory: %w", err)
	}
	data, err := json.MarshalIndent(profileFile{Version: fileVersion, Profiles: profiles}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode software profiles: %w", err)
	}
	data = append(data, '\n')
	temporary, err := os.CreateTemp(directory, ".profiles-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary software profile file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("secure temporary software profile file: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write software profiles: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("flush software profiles: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close software profiles: %w", err)
	}

	backupPath := s.path + ".backup"
	if _, err := os.Stat(s.path); err == nil {
		_ = os.Remove(backupPath)
		if err := os.Rename(s.path, backupPath); err != nil {
			return fmt.Errorf("prepare software profile update: %w", err)
		}
		if err := os.Rename(temporaryPath, s.path); err != nil {
			_ = os.Rename(backupPath, s.path)
			return fmt.Errorf("replace software profile file: %w", err)
		}
		_ = os.Remove(backupPath)
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect software profile file: %w", err)
	}
	if err := os.Rename(temporaryPath, s.path); err != nil {
		return fmt.Errorf("install software profile file: %w", err)
	}
	return nil
}

func newID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("create software profile ID: %w", err)
	}
	return hex.EncodeToString(value), nil
}

func cloneProfiles(profiles []Profile) []Profile {
	result := make([]Profile, len(profiles))
	for index, profile := range profiles {
		result[index] = cloneProfile(profile)
	}
	return result
}

func cloneProfile(profile Profile) Profile {
	profile.Configuration = cloneConfiguration(profile.Configuration)
	return profile
}

func cloneConfiguration(configuration Configuration) Configuration {
	copyOf := configuration
	copyOf.KeyMaps = make([][]protocol.Assignment, len(configuration.KeyMaps))
	for index, mapping := range configuration.KeyMaps {
		copyOf.KeyMaps[index] = append([]protocol.Assignment(nil), mapping...)
	}
	copyOf.CustomLighting.Colors = append([]protocol.KeyColor(nil), configuration.CustomLighting.Colors...)
	copyOf.Actuation = append([]protocol.ActuationSetting(nil), configuration.Actuation...)
	copyOf.MPT = make([][]protocol.MPTStage, len(configuration.MPT))
	for index, stages := range configuration.MPT {
		copyOf.MPT[index] = append([]protocol.MPTStage(nil), stages...)
	}
	copyOf.Macros = make([][]protocol.MacroAction, len(configuration.Macros))
	for slot, actions := range configuration.Macros {
		copyOf.Macros[slot] = make([]protocol.MacroAction, len(actions))
		for index, action := range actions {
			copyOf.Macros[slot][index] = action
			copyOf.Macros[slot][index].Keys = append([]byte(nil), action.Keys...)
		}
	}
	return copyOf
}
