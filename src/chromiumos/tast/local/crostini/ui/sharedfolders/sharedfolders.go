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
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/crostini/faillog"
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

var (
	shareConfirmDialog = nodewith.Name(DialogName).Role(role.Dialog)
	toastNode          = nodewith.Role(role.Alert).ClassName("container")
)

const uiTimeout = 15 * time.Second

type shareConfirmDialogStruct struct {
	Dialog       *nodewith.Finder
	Msg          *nodewith.Finder
	OkButton     *nodewith.Finder
	CancelButton *nodewith.Finder
}

// ShareConfirmDialog represents an instance of the confirm dialog of sharing folder.
var ShareConfirmDialog = shareConfirmDialogStruct{
	Dialog:       shareConfirmDialog,
	Msg:          nodewith.Role(role.StaticText).Ancestor(shareConfirmDialog),
	OkButton:     nodewith.Name("OK").Role(role.Button).Ancestor(shareConfirmDialog),
	CancelButton: nodewith.Name("Cancel").Role(role.Button).Ancestor(shareConfirmDialog),
}

type shareToastNotificationStruct struct {
	Toast        *nodewith.Finder
	Msg          *nodewith.Finder
	ManageButton *nodewith.Finder
}

// ShareToastNotification represents an instance of the toast notification of sharing folder.
var ShareToastNotification = shareToastNotificationStruct{
	Toast:        toastNode,
	Msg:          nodewith.Name("1 folder shared with Linux").Role(role.StaticText).Ancestor(toastNode),
	ManageButton: nodewith.Name("Manage").Role(role.Button),
}

// SharedFolders provides support for actions and records on shared folders.
type SharedFolders struct {
	Folders map[string]struct{}
	ui      *uiauto.Context
	tconn   *chrome.TestConn
}

// NewSharedFolders creates and returns a new Sharefolders instance.
func NewSharedFolders(tconn *chrome.TestConn) *SharedFolders {
	return &SharedFolders{Folders: make(map[string]struct{}), ui: uiauto.New(tconn), tconn: tconn}
}

func (sf *SharedFolders) checkConfirmatioDialog(msg string) uiauto.Action {
	ShareConfirmDialog.Msg = ShareConfirmDialog.Msg.Name(msg)
	return uiauto.Combine("check content of the confirmation dialog",
		sf.ui.WaitUntilExists(ShareConfirmDialog.Dialog),
		sf.ui.WaitUntilExists(ShareConfirmDialog.Msg),
		sf.ui.WaitUntilExists(ShareConfirmDialog.OkButton),
		sf.ui.WaitUntilExists(ShareConfirmDialog.CancelButton))
}

func (sf *SharedFolders) checkToastNotification() uiauto.Action {
	return uiauto.Combine("check content of the toast notification",
		sf.ui.WaitUntilExists(ShareToastNotification.Toast),
		sf.ui.WaitUntilExists(ShareToastNotification.Msg),
		sf.ui.WaitUntilExists(ShareToastNotification.ManageButton))
}

// ShareMyFiles clicks "Share with Linux" on My files.
func (sf *SharedFolders) ShareMyFiles(ctx context.Context, filesApp *filesapp.FilesApp) uiauto.Action {
	return func(ctx context.Context) error {
		if _, ok := sf.Folders[MyFiles]; ok {
			return errors.New("My files has already been shared with Linux")
		}

		return uiauto.Combine("confirm share",
			filesApp.ClickDirectoryContextMenuItem(MyFiles, ShareWithLinux),
			sf.checkConfirmatioDialog(MyFilesMsg))(ctx)
	}
}

// ShareMyFilesOK shares My files and clicks OK on the confirm dialog.
func (sf *SharedFolders) ShareMyFilesOK(ctx context.Context, filesApp *filesapp.FilesApp) uiauto.Action {
	return uiauto.Combine("share My files",
		sf.ShareMyFiles(ctx, filesApp),

		// Click button Ok on the confirmation diaog.
		sf.ui.LeftClick(ShareConfirmDialog.OkButton),

		sf.checkToastNotification(),

		// Record the share.
		sf.AddFolder(MyFiles))
}

// ShareDriveOK shares Google Drive with Crostini and clicks OK button on the confirmation dialog.
func (sf *SharedFolders) ShareDriveOK(ctx context.Context, filesApp *filesapp.FilesApp) uiauto.Action {
	return func(ctx context.Context) error {
		if _, ok := sf.Folders[SharedDrive]; ok {
			return errors.New("Google Drive has already been shared with Linux")
		}

		return uiauto.Combine("share Drive",
			filesApp.ClickDirectoryContextMenuItem(filesapp.GoogleDrive, ShareWithLinux),
			sf.checkConfirmatioDialog(DriveMsg),

			// Click button Ok on the confirmation diaog.
			sf.ui.LeftClick(ShareConfirmDialog.OkButton),

			sf.checkToastNotification(),
			sf.AddFolder(SharedDrive))(ctx)
	}
}

// AddFolder adds shared folders into the map.
func (sf *SharedFolders) AddFolder(folder string) uiauto.Action {
	return func(ctx context.Context) error {
		sf.Folders[folder] = struct{}{}
		return nil
	}
}

// Unshare unshares folders from Linux.
func (sf *SharedFolders) Unshare(cr *chrome.Chrome, folders ...string) uiauto.Action {
	return func(ctx context.Context) (retErr error) {
		s, err := settings.OpenLinuxSettings(ctx, sf.tconn, cr, settings.ManageSharedFolders)
		if err != nil {
			return errors.Wrap(err, "failed to find Manage shared folders")
		}
		defer s.Close(ctx)
		defer func() { faillog.DumpUITreeAndScreenshot(ctx, sf.tconn, "unshare", retErr) }()

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
}

// CheckNoSharedFolders checks there are no folders listed in the Managed shared folders page.
func (sf *SharedFolders) CheckNoSharedFolders(cont *vm.Container, cr *chrome.Chrome) uiauto.Action {
	return func(ctx context.Context) (retErr error) {
		s, err := settings.OpenLinuxSettings(ctx, sf.tconn, cr, settings.ManageSharedFolders)
		if err != nil {
			return errors.Wrap(err, "failed to find Manage shared folders")
		}
		defer s.Close(ctx)
		defer func() { faillog.DumpUITreeAndScreenshot(ctx, sf.tconn, "check_no_shared", retErr) }()

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
}

// UnshareAll unshares all shared folders.
func (sf *SharedFolders) UnshareAll(cont *vm.Container, cr *chrome.Chrome) uiauto.Action {
	return func(ctx context.Context) (retErr error) {
		s, err := settings.OpenLinuxSettings(ctx, sf.tconn, cr, settings.ManageSharedFolders)
		if err != nil {
			return errors.Wrap(err, "failed to open Manage shared folders")
		}
		defer s.Close(ctx)
		defer func() { faillog.DumpUITreeAndScreenshot(ctx, sf.tconn, "unshare_all", retErr) }()

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
}
