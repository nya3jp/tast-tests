// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package settings provides support for the Linux settings on the Settings app.
package settings

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/uig"
	crostiniui "chromiumos/tast/local/crostini/ui"
)

const uiTimeout = 15 * time.Second

// Sub settings name.
const (
	ManageShareFolders = "Manage shared folders"
)

// Window names for different settings page.
const (
	SettingsLinux = "Settings - Linux"
	SettingsMSF   = "Settings - " + ManageShareFolders
)

// Settings represents an instance of the Linux settings in Settings App.
type Settings struct {
	tconn *chrome.TestConn
}

// Open finds or launches Settings app and returns a Settings instance.
// Parameter windowName is the window name of the settings page. If this could not be find,
// it will navigate from Linux (Beta) to Linux settings page.
// Parameter subSettings are sub settings on Linux settings page.
func Open(ctx context.Context, tconn *chrome.TestConn, windowName string, subSettings ...string) (*Settings, error) {
	// Open Settings app.
	if _, err := crostiniui.OpenSettings(ctx, tconn); err != nil {
		return nil, errors.Wrap(err, "failed to open Settings app")
	}

	// Find the specified window.
	_, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Name: windowName, Role: ui.RoleTypeWindow}, uiTimeout)
	settings := &Settings{tconn: tconn}
	if err != nil {
		// Navigate to Linux settings page.
		err = uig.Do(ctx, tconn, uig.Retry(2, uig.FindWithTimeout(ui.FindParams{Role: ui.RoleTypeButton, Name: "Linux (Beta)"}, uiTimeout).FocusAndWait(uiTimeout).LeftClick()))
		if err != nil {
			settings.Close(ctx)
			return nil, errors.Wrap(err, "failed to open Linux settings")
		}

	}

	// Find the sub Settings.
	for _, setting := range subSettings {
		if err := uig.Do(ctx, tconn, uig.FindWithTimeout(ui.FindParams{Name: setting, Role: ui.RoleTypeLink}, uiTimeout).LeftClick()); err != nil {
			settings.Close(ctx)
			return nil, errors.Wrapf(err, "failed to open sub setting %s", setting)
		}
	}

	return settings, nil
}

// Close closes the Settings App.
func (s *Settings) Close(ctx context.Context) error {
	// Close the Settings App.
	if err := apps.Close(ctx, s.tconn, apps.Settings.ID); err != nil {
		return errors.Wrap(err, "failed to close Settings app")
	}

	// Wait for the window to close.
	return ui.WaitUntilGone(ctx, s.tconn, ui.FindParams{Name: "Settings", Role: ui.RoleTypeHeading}, time.Minute)
}

// GetLinuxSharedFolders returns a list of shared folders.
func GetLinuxSharedFolders(ctx context.Context, tconn *chrome.TestConn, windowName string, subSettings ...string) (listOffolders []string, err error) {
	s, err := Open(ctx, tconn, windowName, subSettings...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find Manage shared folders")
	}
	defer s.Close(ctx)

	if err := ui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
		return nil, errors.Wrap(err, "failed to wait for location on Settings app")
	}

	// Find "Shared folders will appear here". It will be displayed if no folder is shared.
	_, textErr := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Name: "Shared folders will appear here", Role: ui.RoleTypeStaticText}, uiTimeout)

	// Find "Shared folders" list. It will be displayed if any folder is shared.
	list, listErr := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Name: "Shared folders", Role: ui.RoleTypeList}, uiTimeout)

	if textErr != nil && listErr != nil {
		// Did not find "Shared folders will appear here" or "Shared folders" list.
		return nil, errors.Wrap(err, "failed to find list of 'Shared folders' or 'Shared folers will appear here'")
	}

	// Method to get shared folders list.
	getList := func() ([]string, error) {
		sharedFolders, err := list.Descendants(ctx, ui.FindParams{Role: ui.RoleTypeButton})
		if err != nil {
			return nil, errors.Wrap(err, "failed to find list of shared folders")
		}
		for _, folder := range sharedFolders {
			listOffolders = append(listOffolders, folder.Name)
		}
		return listOffolders, nil
	}

	if textErr != nil && listErr == nil {
		defer list.Release(ctx)

		// Found "Shared folders".
		return getList()
	}

	// Found "Shared folders will appear here" and "Shared folders" list.
	if listErr == nil {
		defer list.Release(ctx)

		// Unexpectedly found shared folder list.
		listOffolders, err = getList()
		return listOffolders, errors.Wrapf(err, "unexpectedly found Shared folders list %q", listOffolders)
	}

	// No shared folder.
	return nil, nil
}

// UnshareFolder deletes a shared folder from Settings app.
// Settings must be open at the Linux Manage Shared Folders page.
func (s *Settings) UnshareFolder(ctx context.Context, tconn *chrome.TestConn, folder string) error {
	list := uig.FindWithTimeout(ui.FindParams{Name: "Shared folders", Role: ui.RoleTypeList}, uiTimeout)
	folderParam := ui.FindParams{Role: ui.RoleTypeButton, Name: folder}
	if err := uig.Do(ctx, tconn, list); err != nil {
		return errors.Wrap(err, "failed to find shared folder list")
	}

	// Click X button on the target folder.
	if err := uig.Do(ctx, tconn, list.FindWithTimeout(folderParam, uiTimeout).LeftClick()); err != nil {
		return errors.Wrapf(err, "failed to click X button on %s", folder)
	}

	// There might be an unshare dialog. Click OK on it.
	unshareDialog := uig.FindWithTimeout(ui.FindParams{Name: "Unshare failed", Role: ui.RoleTypeDialog}, uiTimeout)
	if err := uig.Do(ctx, tconn, unshareDialog); err == nil {
		if err := uig.Do(ctx, tconn, unshareDialog.FindWithTimeout(ui.FindParams{Name: "OK", Role: ui.RoleTypeButton}, uiTimeout).LeftClick()); err != nil {
			return errors.Wrap(err, "failed to click OK on Unshare failed dialog")
		}
	}

	if err := uig.Do(ctx, tconn, list); err == nil {
		if err := uig.Do(ctx, tconn, list.WaitUntilDescendantGone(folderParam, uiTimeout)); err != nil {
			return errors.Wrapf(err, "%s failed to disappear after deleting", folder)
		}
	}

	return nil
}
