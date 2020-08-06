// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package settingsapp provides support for the Settings app.
package settingsapp

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/mouse"
	crostiniui "chromiumos/tast/local/crostini/ui"
)

const uiTimeout = 15 * time.Second

// SettingsApp represents an instance of the in Settings App.
type SettingsApp struct {
	tconn *chrome.TestConn
	Root  *ui.Node
}

// FindOrLaunch finds or launches Settings app and returns a Settings instance.
func FindOrLaunch(ctx context.Context, tconn *chrome.TestConn, windowName string) (*SettingsApp, error) {
	// Open Settings app.
	if _, err := crostiniui.OpenSettings(ctx, tconn); err != nil {
		return nil, errors.Wrap(err, "failed to open Settings app")
	}

	// Find the Settings app.
	settingsApp, err := FindSettings(ctx, tconn, windowName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find Settings app")
	}
	return settingsApp, nil
}

// FindSettings finds the opened Settings window.
func FindSettings(ctx context.Context, tconn *chrome.TestConn, windowName string) (*SettingsApp, error) {
	// Find the Settings window.
	settings, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Name: windowName, Role: ui.RoleTypeWindow}, uiTimeout)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find Settings window")
	}
	return &SettingsApp{tconn: tconn, Root: settings}, nil
}

// Close closes the Settings App.
func (s *SettingsApp) Close(ctx context.Context) error {
	defer s.Root.Release(ctx)

	// Close the Settings App.
	if err := apps.Close(ctx, s.tconn, apps.Settings.ID); err != nil {
		return err
	}

	// Wait for the window to close.
	return ui.WaitUntilGone(ctx, s.tconn, ui.FindParams{Name: "Settings", Role: ui.RoleTypeHeading}, time.Minute)
}

// openSubSettings opens sub settings on Settings page.
func openSubSettings(ctx context.Context, tconn *chrome.TestConn, params ...ui.FindParams) error {
	for _, param := range params {
		node, err := ui.FindWithTimeout(ctx, tconn, param, uiTimeout)
		if err != nil {
			return errors.Wrapf(err, "failed to find %q", param)
		}
		if err := mouse.Move(ctx, tconn, node.Location.CenterPoint(), time.Second); err != nil {
			return errors.Wrapf(err, "failed to move mouse to %s", node.Name)
		}
		if err := node.LeftClick(ctx); err != nil {
			return errors.Wrapf(err, "failed to left click on %s", node.Name)
		}
	}
	return nil
}

func openLinuxManageSharedFolders(ctx context.Context, tconn *chrome.TestConn) (*SettingsApp, error) {
	// Close Settings app if exists to avoid errors.
	settingsOpen, err := ash.AppShown(ctx, tconn, apps.Settings.ID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to check whether Settings app is running")
	}
	if settingsOpen {
		// Close the Settings App.
		if err := apps.Close(ctx, tconn, apps.Settings.ID); err != nil {
			return nil, errors.Wrap(err, "failed to close the existing Settings app")
		}

		// Wait for the window to close.
		if err := ui.WaitUntilGone(ctx, tconn, ui.FindParams{Name: "Settings", Role: ui.RoleTypeHeading}, time.Minute); err != nil {
			return nil, errors.Wrap(err, "Existing Settings app windows failed to disappear")
		}
	}

	// Launch the Settings app. This one will be used to close Settings app if any error occurs while calling openSubSettings.
	settingsApp, err := FindOrLaunch(ctx, tconn, "Settings")
	if err != nil {
		return nil, errors.Wrap(err, "failed to open Settings app")
	}

	if err := openSubSettings(ctx, tconn,
		ui.FindParams{Name: "Linux (Beta)", Role: ui.RoleTypeLink},
		ui.FindParams{Name: "Linux", Role: ui.RoleTypeStaticText},
		ui.FindParams{Name: "Manage shared folders", Role: ui.RoleTypeLink}); err != nil {
		settingsApp.Close(ctx)
		return nil, errors.Wrap(err, "failed to open sub settings")
	}

	// Get the Settings app again because the window name has changed.
	settingsApp, err = FindOrLaunch(ctx, tconn, "Settings - Manage shared folders")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the Settings app after navigating to Manage shared folders")
	}

	return settingsApp, nil
}

// OpenSettingsAndGetLinuxSharedFolders returns a list of folders shared with Linux.
// To avoid interfering wutg other UI test, Settings app is launched at the beginning of this func and closed before exiting.
// The return array is like this ["My files > Downloads > folder1", "My files > Downloads > folder2"]
func OpenSettingsAndGetLinuxSharedFolders(ctx context.Context, tconn *chrome.TestConn) (listOffolders []string, err error) {
	// Open Settings app.
	settingsApp, err := openLinuxManageSharedFolders(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open Managed Shared folders")
	}

	defer settingsApp.Close(ctx)

	return settingsApp.GetLinuxSharedFolders(ctx, tconn)
}

// GetLinuxSharedFolders returns a list of shared folders.
// Manage shared folders should have been opened before calling this method.
func (s *SettingsApp) GetLinuxSharedFolders(ctx context.Context, tconn *chrome.TestConn) (listOffolders []string, err error) {
	if err := mouse.Move(ctx, tconn, s.Root.Location.CenterPoint(), time.Second); err != nil {
		return nil, errors.Wrap(err, "failed to move mouse to the center of Settings app")
	}

	// Find "Shared folders will appear here". It will be displayed if the no folder is shared.
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
		// Found "Shared folders".
		return getList()
	}

	// Found "Shared folders will appear here" and "Shared folders" list.
	if listErr == nil {
		// Unexpectedly found shared folder list.
		listOffolders, err = getList()
		return listOffolders, errors.Wrapf(err, "unexpectedly found Shared folders list %q", listOffolders)
	}

	// No shared folder.
	return nil, nil
}

// UnshareFoldersFromLinux delete shared folders from Settings app.
func UnshareFoldersFromLinux(ctx context.Context, tconn *chrome.TestConn, sharedFolders ...string) error {
	// Open Settings app.
	settingsApp, err := openLinuxManageSharedFolders(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to open Managed Shared folders")
	}

	defer settingsApp.Close(ctx)

	for _, folder := range sharedFolders {
		// Find "Shared folders" list. It will be displayed if any folder is shared.
		list, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Name: "Shared folders", Role: ui.RoleTypeList}, uiTimeout)
		if err != nil {
			return errors.Wrap(err, "failed to find list of shared folders")
		}

		// Find the target folder in the list.
		folderButton, err := list.DescendantWithTimeout(ctx, ui.FindParams{Role: ui.RoleTypeButton, Name: folder}, uiTimeout)
		if err != nil {
			return errors.Wrapf(err, "failed to find %q in Shared folders list", folder)
		}

		// Right click on the folder to delete it.
		if err := folderButton.LeftClick(ctx); err != nil {
			return errors.Wrapf(err, "failed to left click on the shared folder %s", folder)
		}

		// There might be an unshare dialog. Click on it.
		unshareDialog, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Name: "Unshare failed", Role: ui.RoleTypeDialog}, uiTimeout)
		if err == nil {
			OKButton, err := unshareDialog.DescendantWithTimeout(ctx, ui.FindParams{Name: "OK", Role: ui.RoleTypeButton}, uiTimeout)
			if err != nil {
				return errors.Wrap(err, "failed to find button OK in Unshare failed dialog")
			}
			if err := OKButton.LeftClick(ctx); err != nil {
				return errors.Wrap(err, "failed to click button OK in Unshare failed dialog")
			}
		}

		if err := list.WaitUntilDescendantGone(ctx, ui.FindParams{Role: ui.RoleTypeButton, Name: folder}, uiTimeout); err != nil {
			return errors.Wrapf(err, "%s failed to disappear after deleting", folder)
		}
	}

	return nil
}
