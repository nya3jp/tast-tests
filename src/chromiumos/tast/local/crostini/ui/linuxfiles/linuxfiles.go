// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package linuxfiles supports actions on Linux files on Files app.
package linuxfiles

import (
	"context"
	"reflect"
	"sort"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/chrome/ui/settingsapp"
)

// Linux files directory name and title in Files app.
const (
	DirName = "Linux files"
	Title   = "Files - " + DirName
)

// Folder sharing strings.
const (
	ShareWithLinux     = "Share with Linux"
	ManageLinuxSharing = "Manage Linux Sharing"
	DialogName         = "Share folder with Linux"
	MountPath          = "/mnt/chromeos"
	MountFolderMyfiles = "MyFiles"
	MountPathMyfiles   = MountPath + "/" + MountFolderMyfiles
)

// Strings for sharing My files.
const (
	MyfilesMsg = "Give Linux apps permission to modify files in the My files folder"
	Myfiles    = "My files"
)

// CheckSharingConfirmationDialog checks the content in the folder sharing confirmation dialog.
// OK is clicked if confirmShare is true, otherwise Cancel is clicked.
func CheckSharingConfirmationDialog(ctx context.Context, tconn *chrome.TestConn, filesApp *filesapp.FilesApp, msg string, confirmShare bool) error {
	// There will be a confirmation dialog.
	param := ui.FindParams{
		Name: DialogName,
		Role: ui.RoleTypeDialog,
	}
	dialog, err := filesApp.Root.DescendantWithTimeout(ctx, param, 15*time.Second)
	if err != nil {
		return errors.Wrapf(err, "failed to find confirmation dialog %s", DialogName)
	}

	// Check content of the dialog.
	param = ui.FindParams{
		Name: msg,
		Role: ui.RoleTypeStaticText,
	}
	_, err = dialog.DescendantWithTimeout(ctx, param, 15*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to find confirmation message")
	}

	// Check button Cancel.
	param = ui.FindParams{
		Name: "Cancel",
		Role: ui.RoleTypeButton,
	}
	cancelButton, err := dialog.DescendantWithTimeout(ctx, param, 15*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to find button Cancel in confirmation dialog")
	}

	// Check button OK.
	param = ui.FindParams{
		Name: "OK",
		Role: ui.RoleTypeButton,
	}
	OKButton, err := dialog.DescendantWithTimeout(ctx, param, 15*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to find button Cancel in confirmation dialog")
	}

	// Click button cancel if confirmShare is false.
	if !confirmShare {
		if err := cancelButton.LeftClick(ctx); err != nil {
			return errors.Wrap(err, "failed to click button Cancel")
		}

		// Make sure that there is not sharing alert.
		if _, err := filesApp.Root.DescendantWithTimeout(ctx, ui.FindParams{ClassName: "container", Role: ui.RoleTypeAlert}, 15*time.Second); err == nil {
			return errors.Wrap(err, "alert message is displayed unexpectedly after clicking button Cancel")
		}
		return nil
	}

	// Click button OK.
	if err := OKButton.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click button OK")
	}

	return nil
}

// CheckShareAlert checks the content in the folder sharing alert container.
// Manage is clicked if clickManage is true, otherwise not.
func CheckShareAlert(ctx context.Context, tconn *chrome.TestConn, filesApp *filesapp.FilesApp, clickManage bool) error {
	// Make sure that there is sharing alert.
	if _, err := filesApp.Root.DescendantWithTimeout(ctx, ui.FindParams{ClassName: "container", Role: ui.RoleTypeAlert}, 15*time.Second); err != nil {
		return errors.Wrap(err, "failed to display alert container after clicking button OK")
	}

	// Look for alert message. Other methods would not find the nodes.
	if nodes, err := ui.FindAll(ctx, tconn, ui.FindParams{Name: "1 folder shared with Linux", Role: ui.RoleTypeStaticText}); err != nil || len(nodes) != 1 {
		return errors.Wrap(err, "failed to display alert message after clicking button OK")
	}

	// Look for button Manage. Other methods would not find the node.
	nodes, err := ui.FindAll(ctx, tconn, ui.FindParams{Name: "Manage", Role: ui.RoleTypeButton})
	if err != nil || len(nodes) != 1 {
		return errors.Wrap(err, "failed to display button Manage after clicking button OK")
	}

	if clickManage {
		// Update button Manage.
		if err := nodes[0].Update(ctx); err != nil {
			return errors.Wrap(err, "failed to update button Manager")
		}

		// Move the mouse to the location. It sometimes fails if left click directly.
		if err := mouse.Move(ctx, tconn, nodes[0].Location.CenterPoint(), time.Second); err != nil {
			return errors.Wrap(err, "failed to move mounse to Manager")
		}

		if err := nodes[0].LeftClick(ctx); err != nil {
			return errors.Wrap(err, "failed to click button Manage after sharing")
		}
	}

	return nil
}

// ShareMyfiles tests sharing My files.
func ShareMyfiles(ctx context.Context, tconn *chrome.TestConn, filesApp *filesapp.FilesApp, msg string, confirmShare, clickManage bool) error {
	// Right click My files and select Share with Linux.
	if err := filesApp.SelectContextMenuOfMyfiles(ctx, ShareWithLinux); err != nil {
		return errors.Wrapf(err, "failed to click %q on My files ", ShareWithLinux)
	}

	// Check the confirmation dialog.
	if err := CheckSharingConfirmationDialog(ctx, tconn, filesApp, msg, confirmShare); err != nil {
		return errors.Wrap(err, "failed to check confirmation dialog")
	}

	if confirmShare {
		// Check the alert container.
		if err := CheckShareAlert(ctx, tconn, filesApp, clickManage); err != nil {
			return errors.Wrap(err, "failed to check alert container")
		}
	}

	return nil
}

// CheckSharedFoldersInSettings checks whether the shared folders list in Settings is equal to expected list.
// If Manage shared folders has been opened already, just get the Settings app and verify.
// Otherwise open the Settings app and navigate to Manage shared folders.
func CheckSharedFoldersInSettings(ctx context.Context, tconn *chrome.TestConn, openSettings bool, expectFolders ...string) error {
	var listOffolders []string
	var err error
	if openSettings {
		// Open the Settings app and navigate to Manage shared folders and get the list.
		listOffolders, err = settingsapp.OpenSettingsAndGetLinuxSharedFolders(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get shared folders")
		}
	} else {
		// Get the Settings app first.
		settingsApp, err := settingsapp.FindSettings(ctx, tconn, "Settings - Manage shared folders")
		if err != nil {
			return errors.Wrap(err, "failed to get the Settings app")
		}
		defer settingsApp.Close(ctx)

		// Get the list.
		listOffolders, err = settingsApp.GetLinuxSharedFolders(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get shared folders")
		}
	}

	// Sort and compare the two lists.
	sort.Strings(listOffolders)
	sort.Strings(expectFolders)
	if !reflect.DeepEqual(expectFolders, listOffolders) {
		return errors.Errorf("failed to verify shared folders list, want %, got %s", expectFolders, listOffolders)
	}
	return nil
}
