// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     HomeDirectoryRenameFile,
		Desc:     "Test renaming a file in Linux files and container using a pre-built crostini image",
		Contacts: []string{"jinrongwu@google.com", "cros-containers-dev@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name:              "artifact",
			Pre:               crostini.StartedByArtifact(),
			ExtraData:         []string{crostini.ImageArtifact},
			Timeout:           7 * time.Minute,
			ExtraHardwareDeps: crostini.CrostiniStable,
		}, {
			Name:              "artifact_unstable",
			Pre:               crostini.StartedByArtifact(),
			ExtraData:         []string{crostini.ImageArtifact},
			Timeout:           7 * time.Minute,
			ExtraHardwareDeps: crostini.CrostiniUnstable,
		}, {
			Name:    "download_stretch",
			Pre:     crostini.StartedByDownloadStretch(),
			Timeout: 10 * time.Minute,
		}, {
			Name:    "download_buster",
			Pre:     crostini.StartedByDownloadBuster(),
			Timeout: 10 * time.Minute,
		}},
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}

func HomeDirectoryRenameFile(ctx context.Context, s *testing.State) {
	tconn := s.PreValue().(crostini.PreData).TestAPIConn
	cont := s.PreValue().(crostini.PreData).Container
	keyboard := s.PreValue().(crostini.PreData).Keyboard

	// Use a shortened context for test operations to reserve time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Open Files app.
	filesApp, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to open Files app: ", err)
	}
	defer filesApp.Close(cleanupCtx)

	fileName := "testfile.txt"
	// Create a file in container.
	if err := cont.Command(ctx, "touch", fileName).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to create a file in the container: ", err)
	}

	s.Log("Test renaming a file in Linux files and it should also be renamed in container")
	newFileName := "someotherdgsjtey"
	if err := testRenameFileFromLinuxfiles(ctx, filesApp, cont, keyboard, fileName, newFileName); err != nil {
		s.Fatal("Failed to test Renaming files in Linux files: ", err)
	}

	s.Log("Test renaming a file in container and it should also be renamed in Linux files")
	if err := testRenameFileFromContainer(ctx, filesApp, cont, newFileName+".txt", "newfilename.txt"); err != nil {
		s.Fatal("Failed to test Renaming files in container: ", err)
	}
}

// testRenameFileFromLinuxfiles first renames a file in Linux file then checks it is also renamed in container.
func testRenameFileFromLinuxfiles(ctx context.Context, filesApp *filesapp.FilesApp, cont *vm.Container, keyboard *input.KeyboardEventWriter, fileName, newFileName string) error {
	// Open "Linux files".
	if err := filesApp.OpenDir(ctx, "Linux files", "Files - Linux files"); err != nil {
		return errors.Wrap(err, "failed to open Linux files")
	}

	// Rename a file in Linux files.
	if err := renameFileInLinuxFiles(ctx, filesApp, keyboard, fileName, newFileName); err != nil {
		return errors.Wrapf(err, "failed to rename file %s", fileName)
	}

	// Check the old file does not exist in container.
	if err := cont.CheckFileDoesNotExistInDir(ctx, ".", fileName); err != nil {
		return err
	}

	// Check the new file exists in container.
	if err := cont.CheckFilesExistInDir(ctx, ".", fmt.Sprintf("%s.%s", newFileName, strings.Split(fileName, ".")[1])); err != nil {
		return err
	}
	return nil
}

// testRenameFileFromContainer first renames a file in container then checks it is also renamed in Linux files.
func testRenameFileFromContainer(ctx context.Context, filesApp *filesapp.FilesApp, cont *vm.Container, fileName, newFileName string) error {
	// Rename a file in container.
	if err := cont.Command(ctx, "mv", fileName, newFileName).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "failed to rename file %s in container", fileName)
	}

	// Click Refresh to refresh the file list.
	if err := filesApp.LeftClickItem(ctx, "Refresh", ui.RoleTypeButton); err != nil {
		return errors.Wrapf(err, "failed to click button Refresh on Files app %s ", fileName)
	}

	// The old file should not exist in Linux files.
	if err := checkFileDoesNotExistInLinuxFiles(ctx, filesApp, fileName); err != nil {
		return errors.Wrapf(err, "renamed file %s still exists in Linux files", fileName)
	}
	// The new file should exist in Linux files.
	if err := filesApp.WaitForFile(ctx, newFileName, 10*time.Second); err != nil {
		return errors.Wrapf(err, "file %s is not renamed in Linux files: ", fileName)
	}
	return nil
}

// checkFileDoesNotExistInLinuxFiles checks a file does not exist in Linux files.
// Return error if any occurs or the file exists in Linux files.
func checkFileDoesNotExistInLinuxFiles(ctx context.Context, filesApp *filesapp.FilesApp, fileName string) error {
	// Click Refresh.
	if err := filesApp.LeftClickItem(ctx, "Refresh", ui.RoleTypeButton); err != nil {
		return errors.Wrapf(err, "failed to click button Refresh on Files app %s ", fileName)
	}

	// Check the file has gone.
	return testing.Poll(ctx, func(ctx context.Context) error {
		params := ui.FindParams{
			Name: fileName,
			Role: ui.RoleTypeStaticText,
		}
		if err := filesApp.Root.WaitUntilDescendantGone(ctx, params, 10*time.Second); err != nil {
			return errors.Wrapf(err, "file %s still exists", fileName)
		}
		return nil
	}, &testing.PollOptions{Interval: 100 * time.Millisecond, Timeout: 15 * time.Second})
}

// renameFileInLinuxFiles renames a file in Linux files.
func renameFileInLinuxFiles(ctx context.Context, filesApp *filesapp.FilesApp, keyboard *input.KeyboardEventWriter, oldName, newName string) error {
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

	// Validate the new name.
	if err := keyboard.Accel(ctx, "Enter"); err != nil {
		return errors.Wrapf(err, "failed validating the new name of file %s: ", newName)
	}
	return nil
}
