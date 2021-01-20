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
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

// Folder sharing strings.
const (
	ManageLinuxSharing     = "Manage Linux sharing"
	ShareWithLinux         = `Share with Linux`
	DialogName             = "Share folder with Linux"
	MountPath              = "/mnt/chromeos"
	MountFolderMyFiles     = "MyFiles"
	MountPathMyFiles       = MountPath + "/" + MountFolderMyFiles
	MountFolderGoogleDrive = "GoogleDrive"
	MountPathGoogleDrive   = MountPath + "/" + MountFolderGoogleDrive
	MountFolderMyDrive     = "MyDrive"
	MountPathMyDrive       = MountPathGoogleDrive + "/" + MountFolderMyDrive
	MountPathDownloads     = MountPathMyFiles + "/" + filesapp.Downloads
	MountFolderPlay        = "PlayFiles"
	MountPathPlay          = MountPath + "/" + MountFolderPlay
)

// Strings for sharing My files.
const (
	MyFilesMsg      = "Give Linux apps permission to access files in the My files folder"
	DriveMsg        = "Give Linux apps permission to access files in your Google Drive. Changes will sync to your other devices."
	MyFiles         = "My files"
	SharedDownloads = MyFiles + " › " + filesapp.Downloads

	// SharedDrive represents the name for Drive on the Settings page.
	SharedDrive = filesapp.GoogleDrive + " › " + filesapp.MyDrive
)

const uiTimeout = 15 * time.Second

// ShareConfirmDialog represents the confirm dialog of sharing folder.
type ShareConfirmDialog struct {
	dialogNode   *ui.Node
	msgNode      *ui.Node
	okButton     *ui.Node
	cancelButton *ui.Node
}

// ShareToastNotification represents the toast notification of sharing folder.
type ShareToastNotification struct {
	toastNode    *ui.Node
	msgNode      *ui.Node
	manageButton *ui.Node
}

// SharedFolders provides support for actions and records on shared folders.
type SharedFolders struct {
	Folders map[string]struct{}
}

func findShareConfirmDialog(ctx context.Context, tconn *chrome.TestConn, msg string) (scd *ShareConfirmDialog, err error) {
	// Find the dialog, confirmation message, Cancel and OK button.
	dialog, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Name: DialogName, Role: ui.RoleTypeDialog}, uiTimeout)
	if err != nil {
		return scd, errors.Wrap(err, "failed to find confirmation dialog")
	}
	defer func() {
		if err != nil {
			dialog.Release(ctx)
		}
	}()

	msgNd, err := dialog.DescendantWithTimeout(ctx, ui.FindParams{Name: msg, Role: ui.RoleTypeStaticText}, uiTimeout)
	if err != nil {
		return scd, errors.Wrap(err, "failed to find confirmation message")
	}
	defer func() {
		if err != nil {
			msgNd.Release(ctx)
		}
	}()

	ok, err := dialog.DescendantWithTimeout(ctx, ui.FindParams{Name: "OK", Role: ui.RoleTypeButton}, uiTimeout)
	if err != nil {
		return scd, errors.Wrap(err, "failed to find button OK in confimration dialog")
	}
	defer func() {
		if err != nil {
			ok.Release(ctx)
		}
	}()

	cancel, err := dialog.DescendantWithTimeout(ctx, ui.FindParams{Name: "Cancel", Role: ui.RoleTypeButton}, uiTimeout)
	if err != nil {
		return scd, errors.Wrap(err, "failed to find button Cancel in confimration dialog")
	}
	defer func() {
		if err != nil {
			cancel.Release(ctx)
		}
	}()

	return &ShareConfirmDialog{dialogNode: dialog, msgNode: msgNd, okButton: ok, cancelButton: cancel}, nil
}

// ClickOK clicks OK on the confirm dialog and return an instance of ShareToastNotification.
func (scd *ShareConfirmDialog) ClickOK(ctx context.Context, tconn *chrome.TestConn) (toast *ShareToastNotification, err error) {
	if err := scd.okButton.LeftClick(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to click OK")
	}
	return FindToast(ctx, tconn)
}

// ClickCancel clicks Cancel on the confirm dialog.
func (scd *ShareConfirmDialog) ClickCancel(ctx context.Context, tconn *chrome.TestConn) error {
	if err := scd.cancelButton.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click Cancel")
	}

	// Make sure that there is no sharing toast notification.
	toast, err := FindToast(ctx, tconn)
	if err == nil {
		defer toast.Release(ctx)
		return errors.New("toast notification is displayed unexpectedly after clicking button Cancel")
	}
	return nil
}

// Release releases all nodes on confirm dialog.
func (scd *ShareConfirmDialog) Release(ctx context.Context) {
	scd.cancelButton.Release(ctx)
	scd.okButton.Release(ctx)
	scd.msgNode.Release(ctx)
	scd.dialogNode.Release(ctx)
}

// FindToast finds the share toast notification and checks its content.
func FindToast(ctx context.Context, tconn *chrome.TestConn) (toast *ShareToastNotification, err error) {
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
	return &ShareToastNotification{toastNode: toastNd, msgNode: msg, manageButton: manage}, nil
}

// ClickManage clicks Manage on toast notification.
func (toast *ShareToastNotification) ClickManage(ctx context.Context, tconn *chrome.TestConn) error {
	if err := ui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to wait for location on Files app")
	}

	if err := toast.manageButton.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click button Manage")
	}

	return nil
}

// Release releases all nodes on toast notification.
func (toast *ShareToastNotification) Release(ctx context.Context) {
	toast.manageButton.Release(ctx)
	toast.msgNode.Release(ctx)
	toast.toastNode.Release(ctx)
}

// NewSharedFolders creates and returns a new Sharefolders instance.
func NewSharedFolders() *SharedFolders {
	return &SharedFolders{Folders: make(map[string]struct{})}
}

// ShareMyFiles clicks "Share with Linux" on My files.
// It returns an instance of the confirm dialog.
func (sf *SharedFolders) ShareMyFiles(ctx context.Context, tconn *chrome.TestConn, filesApp *filesapp.FilesApp) (scd *ShareConfirmDialog, err error) {
	if _, ok := sf.Folders[MyFiles]; ok {
		return nil, errors.New("My files has already been shared with Linux")
	}

	// Right click My files and select Share with Linux.
	if err = filesApp.SelectDirectoryContextMenuItem(ctx, MyFiles, ShareWithLinux); err != nil {
		return nil, errors.Wrapf(err, "failed to click %q on My files ", ShareWithLinux)
	}

	return findShareConfirmDialog(ctx, tconn, MyFilesMsg)
}

// ShareMyFilesOK shares My files and clicks OK on the confirm dialog.
func (sf *SharedFolders) ShareMyFilesOK(ctx context.Context, tconn *chrome.TestConn, filesApp *filesapp.FilesApp) error {
	// Share My files, click OK on the confirm dialog, click Manage on the toast nofication.
	scd, err := sf.ShareMyFiles(ctx, tconn, filesApp)
	if err != nil {
		return errors.Wrap(err, "failed to share My files")
	}
	defer scd.Release(ctx)

	// Click button OK.
	toast, err := scd.ClickOK(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to click button OK on share confirm dialog")
	}
	defer toast.Release(ctx)

	sf.AddFolder(MyFiles)

	return nil
}

// ShareDriveOK shares Google Drive with Crostini and clicks OK button on the confirmation dialog.
func (sf *SharedFolders) ShareDriveOK(ctx context.Context, filesApp *filesapp.FilesApp, tconn *chrome.TestConn) error {
	if _, ok := sf.Folders[SharedDrive]; ok {
		return errors.New("Google Drive has already been shared with Linux")
	}

	// Right click Google Drive and select Share with Linux.
	if err := filesApp.SelectDirectoryContextMenuItem(ctx, filesapp.GoogleDrive, ShareWithLinux); err != nil {
		return errors.Wrapf(err, "failed to click %q on Google Drive ", ShareWithLinux)
	}

	scd, err := findShareConfirmDialog(ctx, tconn, DriveMsg)
	if err != nil {
		return errors.Wrap(err, "failed to find share confirm dialog after sharing Google Drive")
	}
	defer scd.Release(ctx)

	// Click button OK.
	toast, err := scd.ClickOK(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to click button OK on share confirm dialog")
	}
	defer toast.Release(ctx)

	sf.AddFolder(SharedDrive)

	return nil
}

// AddFolder adds shared folders into the map.
func (sf *SharedFolders) AddFolder(folder string) {
	sf.Folders[folder] = struct{}{}
}

// Unshare unshares folders from Linux.
func (sf *SharedFolders) Unshare(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome, folders ...string) error {
	s, err := settings.OpenLinuxSettings(ctx, tconn, cr, settings.ManageSharedFolders)
	if err != nil {
		return errors.Wrap(err, "failed to find Manage shared folders")
	}
	defer s.Close(ctx)

	for _, folder := range folders {
		if _, ok := sf.Folders[folder]; !ok {
			return errors.Errorf("%s has not been shared with Linux", folder)
		}
		delete(sf.Folders, folder)
		if err := s.UnshareFolder(ctx, folder); err != nil {
			return errors.Wrapf(err, "failed to unshare %s", folder)
		}
	}

	return nil
}

// CheckNoSharedFolders checks there are no folders listed in the Managed shared folders page.
func (sf *SharedFolders) CheckNoSharedFolders(ctx context.Context, tconn *chrome.TestConn, cont *vm.Container, cr *chrome.Chrome) error {
	s, err := settings.OpenLinuxSettings(ctx, tconn, cr, settings.ManageSharedFolders)
	if err != nil {
		return errors.Wrap(err, "failed to find Manage shared folders")
	}
	defer s.Close(ctx)
	sharedFoldersList, err := s.GetSharedFolders(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to find the shared folders list")
	}
	if sharedFoldersList != nil {
		return errors.Errorf("failed to verify the shared folders list: got %s, want []", sharedFoldersList)
	}

	// Check no shared folders mounted in the container.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		list, err := cont.GetFileList(ctx, MountPath)
		if err != nil {
			return err
		} else if len(list) != 1 || list[0] != "fonts" {
			return errors.Errorf("failed to verify the folders in /mnt/chromeos, got %q, want [fonts]", list)
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to verify file list in container")
	}

	return nil
}

// UnshareAll unshares all shared folders.
func (sf *SharedFolders) UnshareAll(ctx context.Context, tconn *chrome.TestConn, cont *vm.Container, cr *chrome.Chrome) error {
	s, err := settings.OpenLinuxSettings(ctx, tconn, cr, settings.ManageSharedFolders)
	if err != nil {
		return errors.Wrap(err, "failed to open Manage shared folders")
	}
	defer s.Close(ctx)

	sharedFoldersList, err := s.GetSharedFolders(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to find the shared folders list")
	}
	if sharedFoldersList == nil {
		return nil
	}

	for _, folder := range sharedFoldersList {
		if err := s.UnshareFolder(ctx, folder); err != nil {
			return errors.Wrapf(err, "failed to unshare %s", folder)
		}
	}

	sharedFoldersList, err = s.GetSharedFolders(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to find the shared folders list")
	}
	if sharedFoldersList != nil {
		return errors.Errorf("failed to verify the shared folders list: got %s, want []", sharedFoldersList)
	}

	return nil
}
