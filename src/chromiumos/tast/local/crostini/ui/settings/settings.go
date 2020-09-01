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
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/uig"
)

const uiTimeout = 30 * time.Second

// Sub settings name.
const (
	ManageSharedFolders = "Manage shared folders"
)

// Window names for different settings page.
const (
	PageNameLinux = "Settings - Linux"
	PageNameMSF   = "Settings - " + ManageSharedFolders
)

// find params for fixed items.
var (
	linuxBetaButton         = ui.FindParams{Name: "Linux (Beta)", Role: ui.RoleTypeButton}
	linuxIntallerNextButton = ui.FindParams{Name: "Next", Role: ui.RoleTypeButton}
	settingsHeading         = ui.FindParams{Name: "Settings", Role: ui.RoleTypeHeading}
	sharedFoldersMsg        = ui.FindParams{Name: "Shared folders will appear here", Role: ui.RoleTypeStaticText}
	sharedFoldersList       = ui.FindParams{Name: "Shared folders", Role: ui.RoleTypeList}
	unshareFailDlg          = ui.FindParams{Name: "Unshare failed", Role: ui.RoleTypeDialog}
	okButton                = ui.FindParams{Name: "OK", Role: ui.RoleTypeButton}
	removeButton            = ui.FindParams{Name: "Remove Linux for Chromebook", Role: ui.RoleTypeButton}
	removeConfirmDlg        = ui.FindParams{Name: "Delete Linux (Beta)", Role: ui.RoleTypeDialog}
	removeConfirmMsg        = ui.FindParams{Name: "Delete all Linux applications and data in your Linux files folder from this Chromebook?", Role: ui.RoleTypeStaticText}
	deleteButton            = ui.FindParams{Name: "Delete", Role: ui.RoleTypeButton}
	cancelButton            = ui.FindParams{Name: "Cancel", Role: ui.RoleTypeButton}
)

// Settings represents an instance of the Linux settings in Settings App.
type Settings struct {
	tconn *chrome.TestConn
}

// RemoveConfirmDialog represents the confirm dialog of removing Crostini.
type RemoveConfirmDialog struct {
	dialogNode   *ui.Node
	deleteButton *ui.Node
	cancelButton *ui.Node
}

// Open finds or launches Settings app and returns a Settings instance.
func Open(ctx context.Context, tconn *chrome.TestConn) (*Settings, error) {
	// Open Settings app.
	if err := ash.HideAllNotifications(ctx, tconn); err != nil {
		return nil, errors.Wrap(err, "failed to hide all notifications in OpenSettings()")
	}
	s := &Settings{tconn}
	if err := s.ensureOpen(ctx); err != nil {
		return nil, errors.Wrap(err, "error in OpenSettings()")
	}
	return s, nil
}

// OpenLinuxSettings open finds or launches Settings app and navigate to Linux Settings and its sub settings if any.
// Returns a Settings instance.
func OpenLinuxSettings(ctx context.Context, tconn *chrome.TestConn, subSettings ...string) (s *Settings, err error) {
	s, err = Open(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open the Settings app")
	}
	defer func() {
		if err != nil {
			s.Close(ctx)
		}
	}()

	// Navigate to Linux settings page.
	if err = uig.Do(ctx, tconn, uig.Retry(2, uig.FindWithTimeout(linuxBetaButton, uiTimeout).FocusAndWait(uiTimeout).LeftClick())); err != nil {
		return nil, errors.Wrap(err, "failed to open Linux settings")
	}

	// Find the sub Settings.
	for _, setting := range subSettings {
		if err := uig.Do(ctx, tconn, uig.FindWithTimeout(ui.FindParams{Name: setting, Role: ui.RoleTypeLink}, uiTimeout).LeftClick()); err != nil {
			return nil, errors.Wrapf(err, "failed to open sub setting %s", setting)
		}
	}

	return s, nil
}

// FindSettingsPage finds a pre-opened Settings page with a window name.
func FindSettingsPage(ctx context.Context, tconn *chrome.TestConn, windowName string) (s *Settings, err error) {
	// Check Settings app is opened to the specific page.
	if _, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Name: windowName, Role: ui.RoleTypeWindow}, uiTimeout); err != nil {
		return nil, errors.Wrapf(err, "failed to find window %s", windowName)
	}

	return &Settings{tconn: tconn}, nil
}

// ensureOpen checks if the settings app is open, and opens it if it is not.
func (s *Settings) ensureOpen(ctx context.Context) error {
	shown, err := ash.AppShown(ctx, s.tconn, apps.Settings.ID)
	if err != nil {
		return err
	}
	if shown {
		return nil
	}
	if err := apps.Launch(ctx, s.tconn, apps.Settings.ID); err != nil {
		return errors.Wrap(err, "failed to launch settings app")
	}
	if err := ash.WaitForApp(ctx, s.tconn, apps.Settings.ID); err != nil {
		return errors.Wrap(err, "Settings app did not appear in the shelf")
	}
	return nil
}

// OpenInstaller clicks the "Turn on" Linux button to open the Crostini installer.
//
// It also clicks next to skip the information screen.  An ui.Installer
// page object can be constructed after calling OpenInstaller to adjust the settings and to complete the installation.
func (s *Settings) OpenInstaller(ctx context.Context) error {
	if err := s.ensureOpen(ctx); err != nil {
		return errors.Wrap(err, "error in OpenInstaller()")
	}
	return uig.Do(ctx, s.tconn,
		uig.Steps(
			uig.Retry(2, uig.FindWithTimeout(linuxBetaButton, uiTimeout).FocusAndWait(uiTimeout).LeftClick()),
			uig.FindWithTimeout(linuxIntallerNextButton, uiTimeout).LeftClick()).WithNamef("OpenInstaller()"))
}

// Close closes the Settings App.
func (s *Settings) Close(ctx context.Context) error {
	// Close the Settings App.
	if err := apps.Close(ctx, s.tconn, apps.Settings.ID); err != nil {
		return errors.Wrap(err, "failed to close Settings app")
	}

	// Wait for the window to close.
	return ui.WaitUntilGone(ctx, s.tconn, settingsHeading, time.Minute)
}

// GetSharedFolders returns a list of shared folders.
// Settings must be open at the Linux Manage Shared Folders page.
func (s *Settings) GetSharedFolders(ctx context.Context) (listOffolders []string, err error) {
	if err := ui.WaitForLocationChangeCompleted(ctx, s.tconn); err != nil {
		return nil, errors.Wrap(err, "failed to wait for location on Settings app")
	}

	// Find "Shared folders will appear here". It will be displayed if no folder is shared.
	msg, textErr := ui.FindWithTimeout(ctx, s.tconn, sharedFoldersMsg, uiTimeout)
	if msg != nil {
		defer msg.Release(ctx)
	}

	// Find "Shared folders" list. It will be displayed if any folder is shared.
	list, listErr := ui.FindWithTimeout(ctx, s.tconn, sharedFoldersList, uiTimeout)
	if list != nil {
		defer list.Release(ctx)
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

	if textErr != nil && listErr != nil {
		// Did not find "Shared folders will appear here" or "Shared folders" list.
		return nil, errors.Wrap(err, "failed to find list of 'Shared folders' or 'Shared folders will appear here'")
	} else if textErr != nil && listErr == nil {
		// Found "Shared folders".
		return getList()
	} else if listErr == nil {
		// Unexpectedly found shared folder list.
		listOffolders, err = getList()
		return nil, errors.Wrap(err, "unexpectedly found Shared folders list")
	}

	// No shared folder.
	return nil, nil
}

// UnshareFolder deletes a shared folder from Settings app.
// Settings must be open at the Linux Manage Shared Folders page.
func (s *Settings) UnshareFolder(ctx context.Context, folder string) error {
	list := uig.FindWithTimeout(sharedFoldersList, uiTimeout)
	folderParam := ui.FindParams{Role: ui.RoleTypeButton, Name: folder}
	if err := uig.Do(ctx, s.tconn, list); err != nil {
		return errors.Wrap(err, "failed to find shared folder list")
	}

	// Click X button on the target folder.
	if err := uig.Do(ctx, s.tconn, list.FindWithTimeout(folderParam, uiTimeout).LeftClick()); err != nil {
		return errors.Wrapf(err, "failed to click X button on %s", folder)
	}

	// There might be an unshare dialog. Click OK on it.
	unshareDialog := uig.FindWithTimeout(unshareFailDlg, uiTimeout)
	if err := uig.Do(ctx, s.tconn, unshareDialog); err == nil {
		if err := uig.Do(ctx, s.tconn, unshareDialog.FindWithTimeout(okButton, uiTimeout).LeftClick()); err != nil {
			return errors.Wrap(err, "failed to click OK on Unshare failed dialog")
		}
	}

	if err := uig.Do(ctx, s.tconn, list); err == nil {
		if err := uig.Do(ctx, s.tconn, list.WaitUntilDescendantGone(folderParam, uiTimeout)); err != nil {
			return errors.Wrapf(err, "%s failed to disappear after deleting", folder)
		}
	}

	return nil
}

// ClickRemove clicks Remove to delete Linux and returns an instance of RemoveConfirmDialog
func (s *Settings) ClickRemove(ctx context.Context) (*RemoveConfirmDialog, error) {
	if err := uig.Do(ctx, s.tconn,
		uig.FindWithTimeout(removeButton, uiTimeout).LeftClick()); err != nil {
		return nil, errors.Wrap(err, "failed to click Remove")
	}
	return s.findRemoveConfirmDialog(ctx)
}

func (s *Settings) findRemoveConfirmDialog(ctx context.Context) (*RemoveConfirmDialog, error) {
	// Find the dialog, message, Cancel and Delete button.
	dialog, err := ui.FindWithTimeout(ctx, s.tconn, removeConfirmDlg, uiTimeout)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find confirmation dialog")
	}

	msgNd, err := dialog.DescendantWithTimeout(ctx, removeConfirmMsg, uiTimeout)
	if err != nil {
		dialog.Release(ctx)
		return nil, errors.Wrap(err, "failed to find confirmation message")
	}
	defer msgNd.Release(ctx)

	delButton, err := dialog.DescendantWithTimeout(ctx, deleteButton, uiTimeout)
	if err != nil {
		dialog.Release(ctx)
		return nil, errors.Wrap(err, "failed to find button OK in confimration dialog")
	}

	cancel, err := dialog.DescendantWithTimeout(ctx, cancelButton, uiTimeout)
	if err != nil {
		dialog.Release(ctx)
		delButton.Release(ctx)
		return nil, errors.Wrap(err, "failed to find button Cancel in confimration dialog")
	}

	return &RemoveConfirmDialog{dialogNode: dialog, deleteButton: delButton, cancelButton: cancel}, nil
}

// ClickDelete clicks Delete on the remove confirm dialog.
func (rcd *RemoveConfirmDialog) ClickDelete(ctx context.Context, tconn *chrome.TestConn) error {
	// This is necessary, otherwise fails to click Delete.
	if err := ui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to wait for location on Settings app")
	}

	if err := rcd.deleteButton.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click Delete")
	}
	return nil
}

// ClickCancel clicks Cancel on the remove confirm dialog.
func (rcd *RemoveConfirmDialog) ClickCancel(ctx context.Context, tconn *chrome.TestConn) error {
	// This is necessary, otherwise fails to click Cancel.
	if err := ui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to wait for location on Settings app")
	}

	if err := rcd.cancelButton.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click Cancel")
	}

	return nil
}

// Release releases all nodes on the remove confirm dialog.
func (rcd *RemoveConfirmDialog) Release(ctx context.Context) {
	rcd.cancelButton.Release(ctx)
	rcd.deleteButton.Release(ctx)
	rcd.dialogNode.Release(ctx)
}
