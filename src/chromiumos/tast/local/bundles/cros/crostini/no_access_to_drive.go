// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"math/rand"
	"strconv"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     NoAccessToDrive,
		Desc:     "Run a test to make sure linux does not have access to drive on chrome using a pre-built crostini image",
		Contacts: []string{"jinrong@google.com", "cros-containers-dev@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name:              "artifact_gaia",
			Pre:               crostini.StartedByArtifactGaialogin(),
			ExtraData:         []string{crostini.ImageArtifact},
			Timeout:           7 * time.Minute,
			ExtraHardwareDeps: crostini.CrostiniStable,
		}, {
			Name:    "download_buster_gaia",
			Pre:     crostini.StartedByDownloadBusterGaialogin(),
			Timeout: 10 * time.Minute,
		}},
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}

func NoAccessToDrive(ctx context.Context, s *testing.State) {
	tconn := s.PreValue().(crostini.PreData).TestAPIConn
	cont := s.PreValue().(crostini.PreData).Container

	// Use a shortened context for test operations to reserve time for cleanup.
	shortCtx, shortCancel := ctxutil.Shorten(ctx, 15*time.Second)
	defer shortCancel()

	s.Log("Test GoogleDrive are not in the container by default")
	drive := "GoogleDrive"
	mntPath := "/mnt/chromeos"
	if err := cont.CheckFileDoesNotExistInDir(shortCtx, mntPath, drive); err != nil {
		s.Fatal("GoogleDrive is unexpectedly listed in /mnt/chromeos in container: ", err)
	}

	// Open Files app.
	filesApp, err := filesapp.Launch(shortCtx, tconn)
	if err != nil {
		s.Fatal("Failed to open Files app: ", err)
	}
	defer filesApp.Root.Release(shortCtx)

	// Define keyboard to perform keyboard shortcuts.
	keyboard, err := input.Keyboard(shortCtx)
	if err != nil {
		s.Fatal("Error creating keyboard: ", err)
	}
	defer keyboard.Close()

	// Generate a random folder name to avoid duplicate across devices.
	newfolder := "New" + strconv.FormatInt(int64(rand.Intn(10000000000)), 10)
	s.Logf("The new folder name is %s", newfolder)
	// Create a new folder in Drive
	if err := createAFolderInDrive(shortCtx, filesApp, keyboard, newfolder); err != nil {
		s.Fatal("Failed to create a folder in Drive: ", err)
	}

	defer filesApp.DeleteAFileOrFolder(shortCtx, newfolder)

	s.Log("Test home directory in container is empty after creating a folder in Drive in Chrome")
	fileList, err := cont.GetFileList(shortCtx, ".")
	if err != nil {
		s.Fatal("Failed to list the content of home directory in container: ", err)
	}
	if fileList != "" {
		err := errors.Errorf("Home directory unexpectedly contains some files %s", fileList)
		s.Fatal("Home directory in container is not empty after creating a folder in Drive in Chrome: ", err)
	}
	s.Log("Test GoogleDrive are not in the container after creating a folder in Drive in Chrome")
	if err := cont.CheckFileDoesNotExistInDir(shortCtx, mntPath, drive); err != nil {
		s.Fatal("GoogleDrive is unexpectedly listed in /mnt/chromeos in container: ", err)
	}
}

func createAFolderInDrive(shortCtx context.Context, filesApp *filesapp.FilesApp, keyboard *input.KeyboardEventWriter, newfolder string) error {
	// Open Google Drive
	if err := filesApp.OpenDrive(shortCtx); err != nil {
		return errors.Wrap(err, "failed to open Google Drive")
	}
	// Get the Files App listBox.
	filesBox, err := filesApp.Root.DescendantWithTimeout(shortCtx, ui.FindParams{Role: ui.RoleTypeListBox}, 15*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed getting filesBox")
	}
	defer filesBox.Release(shortCtx)

	// Move the focus to the file list.
	if err := filesBox.FocusAndWait(shortCtx, 15*time.Second); err != nil {
		return errors.Wrap(err, "failed selecting filesBox: ")
	}

	// Press ctrl+E to create a folder.
	if err := keyboard.Accel(shortCtx, "ctrl+E"); err != nil {
		return errors.Wrap(err, "failed to create a folder in Google Drive")
	}

	// Wait for rename text field.
	params := ui.FindParams{
		Role:  ui.RoleTypeTextField,
		State: map[ui.StateType]bool{ui.StateTypeEditable: true, ui.StateTypeFocusable: true, ui.StateTypeFocused: true},
	}
	if err := filesApp.Root.WaitUntilDescendantExists(shortCtx, params, 15*time.Second); err != nil {
		return errors.Wrap(err, "failed finding rename input text field")
	}

	// Name the folder with the new name.
	if err := keyboard.Type(shortCtx, newfolder); err != nil {
		return errors.Wrap(err, "failed to rename the new folder")
	}

	// Press Enter.
	if err := keyboard.Accel(shortCtx, "Enter"); err != nil {
		return errors.Wrap(err, "failed to rename the new folder")
	}

	// Check the newly created folder is listed Google Drive.
	if err := filesApp.WaitForFile(shortCtx, newfolder, 10*time.Second); err != nil {
		return errors.Wrapf(err, "failed to list the new folder %s in Drive", newfolder)
	}

	return nil
}
