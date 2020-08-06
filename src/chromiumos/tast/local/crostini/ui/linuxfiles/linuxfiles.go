// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package linuxfiles supports actions on Linux files on Files app.
package linuxfiles

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/crostini/ui/settings"
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
	MountFolderMyFiles = "MyFiles"
	MountPathMyFiles   = MountPath + "/" + MountFolderMyFiles
)

// Strings for sharing My files.
const (
	MyFilesMsg = "Give Linux apps permission to modify files in the My files folder"
	MyFiles    = "My files"
)

const uiTimeout = 15 * time.Second

type shareConfirmDialog struct {
	okButton     *ui.Node
	cancelButton *ui.Node
}

type shareToastNotification struct {
	manageButton *ui.Node
}

// SharedFolders provides support for actions and records on shared folders.
type SharedFolders struct {
	Folders map[string]struct{}
}

func findShareConfirmDialog(ctx context.Context, tconn *chrome.TestConn, msg string) (scd *shareConfirmDialog, err error) {
	// Find the dialog, confirmation message, Cancel and OK button.
	dialog, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Name: DialogName, Role: ui.RoleTypeDialog}, uiTimeout)
	if err != nil {
		return scd, errors.Wrap(err, "failed to find confirmation dialog")
	}
	defer dialog.Release(ctx)

	if _, err := dialog.DescendantWithTimeout(ctx, ui.FindParams{Name: msg, Role: ui.RoleTypeStaticText}, uiTimeout); err != nil {
		return scd, errors.Wrap(err, "failed to find confirmation message")
	}

	ok, err := dialog.DescendantWithTimeout(ctx, ui.FindParams{Name: "OK", Role: ui.RoleTypeButton}, uiTimeout)
	if err != nil {
		return scd, errors.Wrap(err, "failed to find button OK in confimration dialog")
	}

	cancel, err := dialog.DescendantWithTimeout(ctx, ui.FindParams{Name: "Cancel", Role: ui.RoleTypeButton}, uiTimeout)
	if err != nil {
		return scd, errors.Wrap(err, "failed to find button Cancel in confimration dialog")
	}

	return &shareConfirmDialog{okButton: ok, cancelButton: cancel}, nil
}

func (scd *shareConfirmDialog) clickOK(ctx context.Context) error {
	defer scd.okButton.Release(ctx)

	if err := scd.okButton.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click OK")
	}
	return nil
}

func (scd *shareConfirmDialog) clickCancel(ctx context.Context) error {
	defer scd.cancelButton.Release(ctx)

	if err := scd.cancelButton.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click Cancel")
	}
	return nil
}

func findShareToastNotification(ctx context.Context, tconn *chrome.TestConn) (toast *shareToastNotification, err error) {
	// Find the toast notification, message and button Manage.
	toastNode, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{ClassName: "container", Role: ui.RoleTypeAlert}, uiTimeout)
	if err != nil {
		return toast, errors.Wrap(err, "failed to find toast nofication")
	}
	defer toastNode.Release(ctx)

	if _, err := toastNode.DescendantWithTimeout(ctx, ui.FindParams{Name: "1 folder shared with Linux", Role: ui.RoleTypeStaticText}, uiTimeout); err != nil {
		return toast, errors.Wrap(err, "failed to find toast message")
	}

	manage, err := toastNode.DescendantWithTimeout(ctx, ui.FindParams{Name: "Manage", Role: ui.RoleTypeButton}, uiTimeout)
	if err != nil {
		return toast, errors.Wrap(err, "failed to find button Manage in toast notification")
	}
	return &shareToastNotification{manageButton: manage}, nil
}

func (toast *shareToastNotification) clickManage(ctx context.Context, tconn *chrome.TestConn) error {
	defer toast.manageButton.Release(ctx)

	// Move the mouse here because the toast flies from the bottom of Files app to a bit above.
	if err := mouse.Move(ctx, tconn, toast.manageButton.Location.CenterPoint(), time.Second); err != nil {
		return errors.Wrap(err, "failed to move mounse to button Manage")
	}
	if err := toast.manageButton.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click button Manage")
	}

	return nil
}

// CheckShareConfirmDialog checks the content in the folder sharing confirmation dialog.
// OK is clicked if clickOK is true, otherwise Cancel is clicked.
func CheckShareConfirmDialog(ctx context.Context, tconn *chrome.TestConn, msg string, clickOK bool) error {
	scd, err := findShareConfirmDialog(ctx, tconn, msg)
	if err != nil {
		return errors.Wrap(err, "failed to check share confirmation dialog")
	}

	// Click button cancel if clickOK is false.
	if clickOK {
		// Click button OK.
		if err := scd.clickOK(ctx); err != nil {
			return errors.Wrap(err, "failed to click button OK")
		}
	} else {
		if err := scd.clickCancel(ctx); err != nil {
			return errors.Wrap(err, "failed to click button Cancel")
		}

		// Make sure that there is no sharing toast notification.
		if _, err := findShareToastNotification(ctx, tconn); err == nil {
			return errors.New("toast notification is displayed unexpectedly after clicking button Cancel")
		}
	}

	return nil
}

// CheckShareToast checks the content of the sharing toast and clicks Manage if clickManage is true.
func CheckShareToast(ctx context.Context, tconn *chrome.TestConn, clickManage bool) error {
	// Make sure that there is no sharing toast notification.
	toast, err := findShareToastNotification(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to check toast notification")
	}

	if clickManage {
		if err := toast.clickManage(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to click button Manage")
		}
	}

	return nil
}

// ShareMyfiles tests sharing My files.
func (sf *SharedFolders) ShareMyfiles(ctx context.Context, tconn *chrome.TestConn, filesApp *filesapp.FilesApp, msg string, clickOK, clickManage bool) error {
	if _, ok := sf.Folders[MyFiles]; ok {
		return errors.New("My files has already been shared with Linux")
	}
	sf.Folders[MyFiles] = struct{}{}

	// Right click My files and select Share with Linux.
	if err := filesApp.SelectDirectoryContextMenuItem(ctx, MyFiles, ShareWithLinux); err != nil {
		return errors.Wrapf(err, "failed to click %q on My files ", ShareWithLinux)
	}

	// Check the confirmation dialog.
	if err := CheckShareConfirmDialog(ctx, tconn, msg, clickOK); err != nil {
		return errors.Wrap(err, "failed to check confirmation dialog")
	}

	if clickOK {
		// Check the toast notification.
		if err := CheckShareToast(ctx, tconn, clickManage); err != nil {
			return errors.Wrap(err, "failed to check toast notification")
		}
	}

	return nil
}

// Unshare unshares folders from Linux.
func (sf *SharedFolders) Unshare(ctx context.Context, tconn *chrome.TestConn, folders ...string) error {
	s, err := settings.Open(ctx, tconn, settings.SettingsLinux, settings.ManageShareFolders)
	if err != nil || s == nil {
		return errors.Wrap(err, "failed to find Manage shared folders")
	}
	defer s.Close(ctx)

	for _, folder := range folders {
		if _, ok := sf.Folders[MyFiles]; !ok {
			return errors.Errorf("%s has not been shared with Linux", folder)
		}
		delete(sf.Folders, folder)
		if err := settings.UnshareFolder(ctx, tconn, folder); err != nil {
			return errors.Wrapf(err, "failed to unshare %s", folder)
		}
	}

	if len(sf.Folders) == 0 {
		shareFoldersList, err := settings.GetLinuxSharedFolders(ctx, tconn, settings.SettingsMSF)
		if err != nil {
			return errors.Wrap(err, "failed to find the shared folders list")
		}
		if len(shareFoldersList) != 0 {
			return errors.Errorf("failed to verify the shared folders list: want[], got %s", shareFoldersList)
		}
	}

	return nil
}

// UnshareAll unshares all shared folders.
func (sf *SharedFolders) UnshareAll(ctx context.Context, tconn *chrome.TestConn) error {
	folders := make([]string, len(sf.Folders))
	i := 0
	for folder := range sf.Folders {
		folders[i] = folder
		i++
	}
	if err := sf.Unshare(ctx, tconn, folders...); err != nil {
		return errors.Wrap(err, "failed to unshare all shared folders")
	}

	return nil
}
