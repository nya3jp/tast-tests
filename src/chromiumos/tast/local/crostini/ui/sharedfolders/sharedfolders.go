// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package sharedfolders provides support for sharing folders with Crostini.
package sharedfolders

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/crostini/ui/settings"
)

// Folder sharing strings.
const (
	ManageLinuxSharing = "Manage Linux Sharing"
	shareWithLinux     = "Share with Linux"
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
	dialogNode   *ui.Node
	msgNode      *ui.Node
	okButton     *ui.Node
	cancelButton *ui.Node
}

type shareToastNotification struct {
	toastNode    *ui.Node
	msgNode      *ui.Node
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

	msgNd, err := dialog.DescendantWithTimeout(ctx, ui.FindParams{Name: msg, Role: ui.RoleTypeStaticText}, uiTimeout)
	if err != nil {
		defer dialog.Release(ctx)
		return scd, errors.Wrap(err, "failed to find confirmation message")
	}

	ok, err := dialog.DescendantWithTimeout(ctx, ui.FindParams{Name: "OK", Role: ui.RoleTypeButton}, uiTimeout)
	if err != nil {
		defer dialog.Release(ctx)
		defer msgNd.Release(ctx)
		return scd, errors.Wrap(err, "failed to find button OK in confimration dialog")
	}

	cancel, err := dialog.DescendantWithTimeout(ctx, ui.FindParams{Name: "Cancel", Role: ui.RoleTypeButton}, uiTimeout)
	if err != nil {
		defer dialog.Release(ctx)
		defer msgNd.Release(ctx)
		defer ok.Release(ctx)
		return scd, errors.Wrap(err, "failed to find button Cancel in confimration dialog")
	}

	return &shareConfirmDialog{dialogNode: dialog, msgNode: msgNd, okButton: ok, cancelButton: cancel}, nil
}

func (scd *shareConfirmDialog) clickOK(ctx context.Context) error {
	if err := scd.okButton.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click OK")
	}
	return nil
}

func (scd *shareConfirmDialog) clickCancel(ctx context.Context) error {
	if err := scd.cancelButton.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click Cancel")
	}
	return nil
}

func (scd *shareConfirmDialog) Release(ctx context.Context) {
	defer scd.cancelButton.Release(ctx)
	defer scd.okButton.Release(ctx)
	defer scd.msgNode.Release(ctx)
	defer scd.dialogNode.Release(ctx)
}

func findShareToastNotification(ctx context.Context, tconn *chrome.TestConn) (toast *shareToastNotification, err error) {
	// Find the toast notification, message and button Manage.
	toastNd, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{ClassName: "container", Role: ui.RoleTypeAlert}, uiTimeout)
	if err != nil {
		return toast, errors.Wrap(err, "failed to find toast nofication")
	}

	msg, err := toastNd.DescendantWithTimeout(ctx, ui.FindParams{Name: "1 folder shared with Linux", Role: ui.RoleTypeStaticText}, uiTimeout)
	if err != nil {
		defer toastNd.Release(ctx)
		return toast, errors.Wrap(err, "failed to find toast message")
	}

	manage, err := toastNd.DescendantWithTimeout(ctx, ui.FindParams{Name: "Manage", Role: ui.RoleTypeButton}, uiTimeout)
	if err != nil {
		defer toastNd.Release(ctx)
		defer msg.Release(ctx)
		return toast, errors.Wrap(err, "failed to find button Manage in toast notification")
	}
	return &shareToastNotification{toastNode: toastNd, msgNode: msg, manageButton: manage}, nil
}

func (toast *shareToastNotification) clickManage(ctx context.Context, tconn *chrome.TestConn) error {
	if err := ui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to wait for location on Files app")
	}

	if err := toast.manageButton.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click button Manage")
	}

	return nil
}

func (toast *shareToastNotification) Release(ctx context.Context) {
	defer toast.manageButton.Release(ctx)
	defer toast.msgNode.Release(ctx)
	defer toast.toastNode.Release(ctx)
}

// ClickShareConfirmDialog checks the content in the folder sharing confirmation dialog.
// OK is clicked if clickOK is true, otherwise Cancel is clicked.
func ClickShareConfirmDialog(ctx context.Context, tconn *chrome.TestConn, msg string, clickOK bool) error {
	scd, err := findShareConfirmDialog(ctx, tconn, msg)
	if err != nil {
		return errors.Wrap(err, "failed to check share confirmation dialog")
	}
	defer scd.Release(ctx)

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
		toast, err := findShareToastNotification(ctx, tconn)
		if err == nil {
			defer toast.Release(ctx)
			return errors.New("toast notification is displayed unexpectedly after clicking button Cancel")
		}
	}

	return nil
}

// ClickShareToast checks the content of the sharing toast and clicks Manage if clickManage is true.
func ClickShareToast(ctx context.Context, tconn *chrome.TestConn, clickManage bool) error {
	// Make sure that there is no sharing toast notification.
	toast, err := findShareToastNotification(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to check toast notification")
	}
	defer toast.Release(ctx)

	if clickManage {
		if err := toast.clickManage(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to click button Manage")
		}
	}

	return nil
}

// ShareMyFiles tests sharing My files.
func (sf *SharedFolders) ShareMyFiles(ctx context.Context, tconn *chrome.TestConn, filesApp *filesapp.FilesApp, msg string, clickOK, clickManage bool) error {
	if _, ok := sf.Folders[MyFiles]; ok {
		return errors.New("My files has already been shared with Linux")
	}
	sf.Folders[MyFiles] = struct{}{}

	// Right click My files and select Share with Linux.
	if err := filesApp.SelectDirectoryContextMenuItem(ctx, MyFiles, shareWithLinux); err != nil {
		return errors.Wrapf(err, "failed to click %q on My files ", shareWithLinux)
	}

	// Check the confirmation dialog.
	if err := ClickShareConfirmDialog(ctx, tconn, msg, clickOK); err != nil {
		return errors.Wrap(err, "failed to check confirmation dialog")
	}

	if clickOK {
		// Check the toast notification.
		if err := ClickShareToast(ctx, tconn, clickManage); err != nil {
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
		if err := s.UnshareFolder(ctx, tconn, folder); err != nil {
			return errors.Wrapf(err, "failed to unshare %s", folder)
		}
	}

	return nil
}

// CheckNoSharedFolders checks there are no folders listed in the Managed shared folders page.
func (sf *SharedFolders) CheckNoSharedFolders(ctx context.Context, tconn *chrome.TestConn) error {
	s, err := settings.Open(ctx, tconn, settings.SettingsLinux, settings.ManageShareFolders)
	if err != nil || s == nil {
		return errors.Wrap(err, "failed to find Manage shared folders")
	}
	defer s.Close(ctx)

	shareFoldersList, err := settings.GetLinuxSharedFolders(ctx, tconn, settings.SettingsMSF)
	if err != nil {
		return errors.Wrap(err, "failed to find the shared folders list")
	}
	if len(shareFoldersList) != 0 {
		return errors.Errorf("failed to verify the shared folders list: want[], got %s", shareFoldersList)
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

	if err := sf.CheckNoSharedFolders(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to verify that the shared folder list is empty")
	}

	return nil
}
