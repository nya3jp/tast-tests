// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package sharedfolders provides support for sharing folders with Crostini.
package sharedfolders

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/crostini/ui/linuxfiles"
	"chromiumos/tast/local/crostini/ui/settings"
)

const shareWithLinux = "Share with Linux"

// SharedFolders provides support for actions and records on shared folders.
type SharedFolders struct {
	Folders map[string]struct{}
}

// ShareMyfiles tests sharing My files.
func (sf *SharedFolders) ShareMyfiles(ctx context.Context, tconn *chrome.TestConn, filesApp *filesapp.FilesApp, msg string, clickOK, clickManage bool) error {
	if _, ok := sf.Folders[linuxfiles.MyFiles]; ok {
		return errors.New("My files has already been shared with Linux")
	}
	sf.Folders[linuxfiles.MyFiles] = struct{}{}

	// Right click My files and select Share with Linux.
	if err := filesApp.SelectDirectoryContextMenuItem(ctx, linuxfiles.MyFiles, shareWithLinux); err != nil {
		return errors.Wrapf(err, "failed to click %q on My files ", shareWithLinux)
	}

	// Check the confirmation dialog.
	if err := linuxfiles.CheckShareConfirmDialog(ctx, tconn, msg, clickOK); err != nil {
		return errors.Wrap(err, "failed to check confirmation dialog")
	}

	if clickOK {
		// Check the toast notification.
		if err := linuxfiles.CheckShareToast(ctx, tconn, clickManage); err != nil {
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
		if _, ok := sf.Folders[linuxfiles.MyFiles]; !ok {
			return errors.Errorf("%s has not been shared with Linux", folder)
		}
		delete(sf.Folders, folder)
		if err := s.UnshareFolder(ctx, tconn, folder); err != nil {
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
