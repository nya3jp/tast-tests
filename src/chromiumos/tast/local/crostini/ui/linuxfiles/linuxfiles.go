// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package linuxfiles supports actions on Linux files on Files app.
package linuxfiles

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/input"
)

// Linux files name in Files app
const (
	LinuxFiles      = "Linux files"
	FilesLinuxFiles = "Files - " + LinuxFiles
)

// CheckFileDoesNotExist checks a file does not exist in Linux files.
// Return error if any occurs or the file exists in Linux files.
func CheckFileDoesNotExist(ctx context.Context, filesApp *filesapp.FilesApp, fileName string) error {
	// Open "Linux files".
	if err := filesApp.OpenDir(ctx, LinuxFiles, FilesLinuxFiles); err != nil {
		return errors.Wrap(err, "failed to open Linux files")
	}

	// Click Refresh.
	if err := filesApp.LeftClickItem(ctx, "Refresh", ui.RoleTypeButton); err != nil {
		return errors.Wrapf(err, "failed to click button Refresh on Files app %s ", fileName)
	}

	// Check the file has gone.
	params := ui.FindParams{
		Name: fileName,
		Role: ui.RoleTypeStaticText,
	}
	if err := filesApp.Root.WaitUntilDescendantGone(ctx, params, 20*time.Second); err != nil {
		return errors.Wrapf(err, "file %s still exists", fileName)
	}
	return nil
}

// RenameFile renames a file in Linux files.
func RenameFile(ctx context.Context, filesApp *filesapp.FilesApp, keyboard *input.KeyboardEventWriter, oldName, newName string) error {
	// Open "Linux files".
	if err := filesApp.OpenDir(ctx, LinuxFiles, FilesLinuxFiles); err != nil {
		return errors.Wrap(err, "failed to open Linux files")
	}

	// Right click and select rename.
	if err := filesApp.SelectContextMenu(ctx, oldName, filesapp.Rename); err != nil {
		return errors.Wrapf(err, "failed to select Rename in context menu for file %s in Linux files", oldName)
	}

	// Wait for rename text field.
	params := ui.FindParams{
		Role:  ui.RoleTypeTextField,
		State: map[ui.StateType]bool{ui.StateTypeEditable: true, ui.StateTypeFocusable: true, ui.StateTypeFocused: true},
	}
	if err := filesApp.Root.WaitUntilDescendantExists(ctx, params, 15*time.Second); err != nil {
		return errors.Wrap(err, "failed finding rename input text field")
	}

	// Type the new name.
	if err := keyboard.Type(ctx, newName); err != nil {
		return errors.Wrapf(err, "failed to rename the file %s", oldName)
	}

	// Press Enter.
	if err := keyboard.Accel(ctx, "Enter"); err != nil {
		return errors.Wrapf(err, "failed validating the new name of file %s: ", newName)
	}
	return nil
}
