package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/joseferrao/ducky-drv/internal/profiles"
	"github.com/joseferrao/ducky-drv/internal/protocol"
)

const softwareProfileTimeout = 3 * time.Minute

type softwareProfileControls struct {
	root     fyne.CanvasObject
	list     *widget.List
	details  *widget.Label
	profiles []profiles.Profile
	selected int
	create   *widget.Button
	load     *widget.Button
	update   *widget.Button
	rename   *widget.Button
	delete   *widget.Button
}

func (u *UI) buildSoftwareProfiles() *softwareProfileControls {
	t := u.i18n.T
	controls := &softwareProfileControls{selected: -1}
	if u.profileStore != nil {
		controls.profiles = u.profileStore.Profiles()
	}
	controls.details = widget.NewLabel(t("software_profiles.no_selection"))
	controls.details.Wrapping = fyne.TextWrapWord
	controls.list = widget.NewList(
		func() int { return len(controls.profiles) },
		func() fyne.CanvasObject { return widget.NewLabel("Profile") },
		func(id widget.ListItemID, object fyne.CanvasObject) {
			if id < 0 || id >= len(controls.profiles) {
				return
			}
			profile := controls.profiles[id]
			object.(*widget.Label).SetText(fmt.Sprintf(
				"%s   ·   %s", profile.Name, profile.UpdatedAt.Local().Format("2006-01-02 15:04"),
			))
		},
	)
	controls.list.OnSelected = func(id widget.ListItemID) {
		controls.selected = int(id)
		u.updateSoftwareProfileDetails(controls)
		u.updateSoftwareProfileActions()
	}
	controls.list.OnUnselected = func(widget.ListItemID) {
		controls.selected = -1
		controls.details.SetText(t("software_profiles.no_selection"))
		u.updateSoftwareProfileActions()
	}

	controls.create = widget.NewButtonWithIcon(t("software_profiles.create"), theme.ContentAddIcon(), u.openCreateSoftwareProfile)
	controls.load = widget.NewButtonWithIcon(t("software_profiles.load"), theme.UploadIcon(), u.confirmLoadSoftwareProfile)
	controls.load.Importance = widget.HighImportance
	controls.update = widget.NewButtonWithIcon(t("software_profiles.update"), theme.ViewRefreshIcon(), u.confirmUpdateSoftwareProfile)
	controls.rename = widget.NewButtonWithIcon(t("software_profiles.rename"), theme.DocumentCreateIcon(), u.openRenameSoftwareProfile)
	controls.delete = widget.NewButtonWithIcon(t("software_profiles.delete"), theme.DeleteIcon(), u.confirmDeleteSoftwareProfile)
	controls.delete.Importance = widget.DangerImportance

	description := widget.NewLabel(t("software_profiles.description"))
	description.Wrapping = fyne.TextWrapWord
	target := widget.NewLabel(t("software_profiles.target_hint"))
	target.Wrapping = fyne.TextWrapWord
	target.Importance = widget.LowImportance
	path := ""
	if u.profileStore != nil {
		path = u.profileStore.Path()
	}
	file := widget.NewLabel(fmt.Sprintf(t("software_profiles.file"), path))
	file.Wrapping = fyne.TextWrapBreak
	file.Importance = widget.LowImportance

	toolbar := container.NewHBox(controls.create, controls.load, controls.update, controls.rename, controls.delete)
	listCard := widget.NewCard(t("software_profiles.library"), "", controls.list)
	controls.root = container.NewBorder(
		container.NewVBox(description, target, widget.NewSeparator(), toolbar),
		container.NewVBox(widget.NewSeparator(), controls.details, file),
		nil, nil,
		container.NewPadded(listCard),
	)
	return controls
}

func (u *UI) selectedSoftwareProfile() (profiles.Profile, bool) {
	if u.softwareProfiles == nil {
		return profiles.Profile{}, false
	}
	index := u.softwareProfiles.selected
	if index < 0 || index >= len(u.softwareProfiles.profiles) {
		return profiles.Profile{}, false
	}
	return u.softwareProfiles.profiles[index], true
}

func (u *UI) updateSoftwareProfileDetails(controls *softwareProfileControls) {
	if controls == nil || controls.selected < 0 || controls.selected >= len(controls.profiles) {
		return
	}
	profile := controls.profiles[controls.selected]
	controls.details.SetText(fmt.Sprintf(
		u.i18n.T("software_profiles.selected_detail"),
		profile.Name, profile.UpdatedAt.Local().Format("2006-01-02 15:04"),
	))
}

func (u *UI) updateSoftwareProfileActions() {
	controls := u.softwareProfiles
	if controls == nil || controls.create == nil {
		return
	}
	connected := u.client != nil && !u.busy
	selected := controls.selected >= 0 && controls.selected < len(controls.profiles)
	setButtonEnabled(controls.create, connected)
	setButtonEnabled(controls.load, connected && selected)
	setButtonEnabled(controls.update, connected && selected)
	setButtonEnabled(controls.rename, !u.busy && selected)
	setButtonEnabled(controls.delete, !u.busy && selected)
}

func setButtonEnabled(button *widget.Button, enabled bool) {
	if enabled {
		button.Enable()
	} else {
		button.Disable()
	}
}

func (u *UI) openCreateSoftwareProfile() {
	if u.client == nil || u.busy {
		return
	}
	entry := widget.NewEntry()
	entry.SetPlaceHolder(u.i18n.T("software_profiles.name_placeholder"))
	entry.SetText(fmt.Sprintf(u.i18n.T("software_profiles.default_name"), len(u.softwareProfiles.profiles)+1))
	form := widget.NewForm(widget.NewFormItem(u.i18n.T("software_profiles.name"), entry))
	modal := dialog.NewCustomConfirm(
		u.i18n.T("software_profiles.create"), u.i18n.T("software_profiles.create_action"), u.i18n.T("action.cancel"), form,
		func(ok bool) {
			if !ok {
				return
			}
			name := strings.TrimSpace(entry.Text)
			if name == "" {
				dialog.ShowInformation(u.i18n.T("error.invalid"), u.i18n.T("software_profiles.name_required"), u.window)
				return
			}
			u.captureSoftwareProfile(name, "")
		}, u.window,
	)
	modal.Resize(fyne.NewSize(440, 190))
	modal.Show()
}

func (u *UI) confirmUpdateSoftwareProfile() {
	profile, ok := u.selectedSoftwareProfile()
	if !ok || u.client == nil || u.busy {
		return
	}
	dialog.ShowConfirm(
		u.i18n.T("software_profiles.update"),
		fmt.Sprintf(u.i18n.T("software_profiles.update_confirm"), profile.Name, u.profileSelect.Selected),
		func(confirmed bool) {
			if confirmed {
				u.captureSoftwareProfile(profile.Name, profile.ID)
			}
		}, u.window,
	)
}

func (u *UI) captureSoftwareProfile(name, updateID string) {
	u.cancelAutoSync()
	targetProfile := u.selectedMemoryProfile()
	u.runProfileTask(func(ctx context.Context) (func(), error) {
		if err := u.client.SetProfile(ctx, targetProfile); err != nil {
			return nil, fmt.Errorf("select target memory profile: %w", err)
		}
		configuration, err := u.readCompleteConfiguration(ctx, u.client)
		if err != nil {
			return nil, err
		}
		var saved profiles.Profile
		if updateID == "" {
			saved, err = u.profileStore.Create(name, configuration)
		} else {
			saved, err = u.profileStore.Update(updateID, configuration)
		}
		if err != nil {
			return nil, err
		}
		return func() {
			u.refreshSoftwareProfiles(saved.ID)
			u.detail.SetText(fmt.Sprintf(u.i18n.T("software_profiles.saved_status"), saved.Name))
		}, nil
	})
}

func (u *UI) confirmLoadSoftwareProfile() {
	profile, ok := u.selectedSoftwareProfile()
	if !ok || u.client == nil || u.busy {
		return
	}
	dialog.ShowConfirm(
		u.i18n.T("software_profiles.load"),
		fmt.Sprintf(u.i18n.T("software_profiles.load_confirm"), profile.Name, u.profileSelect.Selected),
		func(confirmed bool) {
			if confirmed {
				u.loadSoftwareProfile(profile)
			}
		}, u.window,
	)
}

func (u *UI) loadSoftwareProfile(profile profiles.Profile) {
	u.cancelAutoSync()
	targetProfile := u.selectedMemoryProfile()
	u.runProfileTask(func(ctx context.Context) (func(), error) {
		if err := u.client.SetProfile(ctx, targetProfile); err != nil {
			return nil, fmt.Errorf("select target memory profile: %w", err)
		}
		if err := u.writeCompleteConfiguration(ctx, u.client, profile.Configuration); err != nil {
			return nil, fmt.Errorf("%w; the selected keyboard memory profile may be partially updated", err)
		}
		return func() {
			u.withoutAutoSync(func() {
				u.applyConfiguration(u.configurationSnapshotFromProfile(profile.Configuration))
			})
			u.detail.SetText(fmt.Sprintf(
				u.i18n.T("software_profiles.loaded_status"), profile.Name, u.profileSelect.Selected,
			))
		}, nil
	})
}

func (u *UI) openRenameSoftwareProfile() {
	profile, ok := u.selectedSoftwareProfile()
	if !ok || u.busy {
		return
	}
	entry := widget.NewEntry()
	entry.SetText(profile.Name)
	form := widget.NewForm(widget.NewFormItem(u.i18n.T("software_profiles.name"), entry))
	modal := dialog.NewCustomConfirm(
		u.i18n.T("software_profiles.rename"), u.i18n.T("software_profiles.rename_action"), u.i18n.T("action.cancel"), form,
		func(confirmed bool) {
			if !confirmed {
				return
			}
			renamed, err := u.profileStore.Rename(profile.ID, entry.Text)
			if err != nil {
				u.showSoftwareProfileError(err)
				return
			}
			u.refreshSoftwareProfiles(renamed.ID)
		}, u.window,
	)
	modal.Resize(fyne.NewSize(440, 190))
	modal.Show()
}

func (u *UI) confirmDeleteSoftwareProfile() {
	profile, ok := u.selectedSoftwareProfile()
	if !ok || u.busy {
		return
	}
	dialog.ShowConfirm(
		u.i18n.T("software_profiles.delete"),
		fmt.Sprintf(u.i18n.T("software_profiles.delete_confirm"), profile.Name),
		func(confirmed bool) {
			if !confirmed {
				return
			}
			if err := u.profileStore.Delete(profile.ID); err != nil {
				u.showSoftwareProfileError(err)
				return
			}
			u.refreshSoftwareProfiles("")
			u.detail.SetText(u.i18n.T("software_profiles.deleted_status"))
		}, u.window,
	)
}

func (u *UI) refreshSoftwareProfiles(selectID string) {
	if u.profileStore == nil || u.softwareProfiles == nil || u.softwareProfiles.list == nil {
		return
	}
	u.softwareProfiles.profiles = u.profileStore.Profiles()
	u.softwareProfiles.selected = -1
	u.softwareProfiles.list.UnselectAll()
	u.softwareProfiles.list.Refresh()
	if selectID != "" {
		for index, profile := range u.softwareProfiles.profiles {
			if profile.ID == selectID {
				u.softwareProfiles.list.Select(widget.ListItemID(index))
				break
			}
		}
	}
	if u.softwareProfiles.selected < 0 {
		u.softwareProfiles.details.SetText(u.i18n.T("software_profiles.no_selection"))
	}
	u.updateSoftwareProfileActions()
}

func (u *UI) showSoftwareProfileError(err error) {
	dialog.ShowError(fmt.Errorf("%s: %w", u.i18n.T("software_profiles.error"), err), u.window)
}

func (u *UI) runProfileTask(work func(context.Context) (func(), error)) {
	if u.client == nil {
		dialog.ShowInformation(u.i18n.T("action.connect"), u.i18n.T("error.no_device"), u.window)
		return
	}
	if u.busy {
		return
	}
	u.setBusy(true, u.i18n.T("software_profiles.working"))
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), softwareProfileTimeout)
		defer cancel()
		apply, err := work(ctx)
		fyne.Do(func() {
			u.setBusy(false, "")
			if err != nil {
				u.showSoftwareProfileError(err)
				return
			}
			if apply != nil {
				apply()
			}
		})
	}()
}

func (u *UI) readCompleteConfiguration(ctx context.Context, client *protocol.Client) (configuration profiles.Configuration, resultErr error) {
	activeLayer, err := client.ActiveLayer(ctx)
	if err != nil {
		return configuration, fmt.Errorf("read active layer: %w", err)
	}
	if activeLayer < 0 || activeLayer > 1 {
		activeLayer = 0
	}
	configuration.ActiveLayer = activeLayer
	defer func() {
		if err := client.SetLayer(ctx, activeLayer); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("restore active layer: %w", err))
		}
	}()

	configuration.KeyMaps = make([][]protocol.Assignment, 2)
	for layer := range configuration.KeyMaps {
		if err := client.SetLayer(ctx, layer); err != nil {
			return configuration, fmt.Errorf("select key layer %d: %w", layer, err)
		}
		configuration.KeyMaps[layer], err = client.KeyMap(ctx, layer)
		if err != nil {
			return configuration, fmt.Errorf("read key layer %d: %w", layer, err)
		}
	}
	configuration.Lighting, err = client.Lighting(ctx)
	if err != nil {
		return configuration, fmt.Errorf("read lighting: %w", err)
	}
	if configuration.Lighting.Effect == protocol.CustomStaticLightingEffect {
		configuration.CustomLighting, err = client.CustomLighting(ctx)
		if err != nil {
			return configuration, fmt.Errorf("read custom lighting: %w", err)
		}
		configuration.Lighting.Brightness = configuration.CustomLighting.Brightness
	}
	configuration.Actuation, err = client.Actuation(ctx)
	if err != nil {
		return configuration, fmt.Errorf("read actuation: %w", err)
	}
	configuration.MPT = make([][]protocol.MPTStage, protocol.MPTPresets)
	for preset := range configuration.MPT {
		configuration.MPT[preset], err = client.MPT(ctx, preset)
		if err != nil {
			return configuration, fmt.Errorf("read MPT%d: %w", preset+1, err)
		}
	}
	configuration.Macros = make([][]protocol.MacroAction, protocol.MacroSlots)
	for slot := range configuration.Macros {
		configuration.Macros[slot], err = client.Macro(ctx, slot+1)
		if err != nil {
			return configuration, fmt.Errorf("read macro M%d: %w", slot+1, err)
		}
	}
	if err := profiles.Validate(configuration); err != nil {
		return configuration, err
	}
	return configuration, nil
}

func (u *UI) writeCompleteConfiguration(ctx context.Context, client *protocol.Client, configuration profiles.Configuration) error {
	if err := profiles.Validate(configuration); err != nil {
		return err
	}
	for layer, mapping := range configuration.KeyMaps {
		if err := client.SetKeyMap(ctx, layer, mapping); err != nil {
			return fmt.Errorf("write key layer %d: %w", layer, err)
		}
	}
	if configuration.Lighting.Effect == protocol.CustomStaticLightingEffect {
		if err := client.SetCustomLighting(ctx, configuration.CustomLighting); err != nil {
			return fmt.Errorf("write custom lighting: %w", err)
		}
	} else if err := client.SetLighting(ctx, configuration.Lighting); err != nil {
		return fmt.Errorf("write lighting: %w", err)
	}
	if err := client.SetActuation(ctx, configuration.Actuation); err != nil {
		return fmt.Errorf("write actuation: %w", err)
	}
	for preset, stages := range configuration.MPT {
		if err := client.SetMPT(ctx, preset, stages); err != nil {
			return fmt.Errorf("write MPT%d: %w", preset+1, err)
		}
	}
	for slot, actions := range configuration.Macros {
		if err := client.SetMacro(ctx, slot+1, actions); err != nil {
			return fmt.Errorf("write macro M%d: %w", slot+1, err)
		}
	}
	if err := client.SetLayer(ctx, configuration.ActiveLayer); err != nil {
		return fmt.Errorf("select saved active layer: %w", err)
	}
	return nil
}

func (u *UI) configurationSnapshotFromProfile(configuration profiles.Configuration) configurationSnapshot {
	layer := configuration.ActiveLayer
	if layer < 0 || layer >= len(configuration.KeyMaps) {
		layer = 0
	}
	snapshot := configurationSnapshot{
		layer: layer, lighting: configuration.Lighting, custom: configuration.CustomLighting,
		actuation: append([]protocol.ActuationSetting(nil), configuration.Actuation...),
	}
	if layer < len(configuration.KeyMaps) {
		snapshot.mapping = append([]protocol.Assignment(nil), configuration.KeyMaps[layer]...)
	}
	preset := parsePreset(u.mpt.preset.Selected)
	if preset < 0 || preset >= len(configuration.MPT) {
		preset = 0
	}
	if len(configuration.MPT) > 0 {
		snapshot.mpt = append([]protocol.MPTStage(nil), configuration.MPT[preset]...)
	}
	slot := macroSlot(u.macros.slot.Selected) - 1
	if slot < 0 || slot >= len(configuration.Macros) {
		slot = 0
	}
	if len(configuration.Macros) > 0 {
		snapshot.macros = append([]protocol.MacroAction(nil), configuration.Macros[slot]...)
	}
	return snapshot
}

func (u *UI) selectedMemoryProfile() int {
	if u.profileSelect != nil && u.profileSelect.Selected == u.i18n.T("profile.2") {
		return 1
	}
	return 0
}
