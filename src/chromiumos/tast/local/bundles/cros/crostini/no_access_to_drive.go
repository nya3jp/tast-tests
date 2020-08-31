// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	drive   = "GoogleDrive"
	mntPath = "/mnt/chromeos"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     NoAccessToDrive,
		Desc:     "Run a test to make sure crostini does not have access to GoogleDrive",
		Contacts: []string{"jinrong@google.com", "cros-containers-dev@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		Vars:     []string{"crostini.gaiaUsername", "crostini.gaiaPassword", "crostini.gaiaID", "keepState"},
		Params: []testing.Param{{
			Name:              "artifact_gaia",
			Pre:               crostini.StartedByArtifactWithGaiaLogin(),
			ExtraData:         []string{crostini.ImageArtifact},
			Timeout:           7 * time.Minute,
			ExtraHardwareDeps: crostini.CrostiniStable,
		}, {
			Name:    "download_buster_gaia",
			Pre:     crostini.StartedByDownloadBusterWithGaiaLogin(),
			Timeout: 10 * time.Minute,
		}},
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}

func NoAccessToDrive(ctx context.Context, s *testing.State) {
	tconn := s.PreValue().(crostini.PreData).TestAPIConn
	cont := s.PreValue().(crostini.PreData).Container
	defer crostini.RunCrostiniPostTest(ctx,
		s.PreValue().(crostini.PreData).Container,
		s.PreValue().(crostini.PreData).Chrome.User())

	// Use a shortened context for test operations to reserve time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	if err := cont.CheckFileDoesNotExistInDir(ctx, mntPath, drive); err != nil {
		s.Fatalf("GoogleDrive is unexpectedly listed in %s in the container: %s", mntPath, err)
	}

	// Open Files app.
	filesApp, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to open Files app: ", err)
	}
	defer filesApp.Root.Release(cleanupCtx)

	// Define keyboard to perform keyboard shortcuts.
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Error creating keyboard: ", err)
	}
	defer keyboard.Close()

	// Generate a random folder name to avoid duplicate across devices.
	newFolder := fmt.Sprintf("NoAccessToDrive_%d", rand.Intn(1000000000))
	s.Log("The new folder name is ", newFolder)
	// Create a new folder in Drive
	if err := createFolderInDrive(ctx, filesApp, keyboard, newFolder); err != nil {
		s.Fatal("Failed to create a folder in Drive: ", err)
	}

	defer filesApp.DeleteFileOrFolder(cleanupCtx, newFolder)

	fileList, err := cont.GetFileList(ctx, ".")
	if err != nil {
		s.Fatal("Failed to list the content of home directory in container: ", err)
	}
	if len(fileList) != 0 {
		s.Fatalf("Failed to verify file list in home directory in the container: got %q, want []", fileList)
	}

	if err := cont.CheckFileDoesNotExistInDir(ctx, mntPath, drive); err != nil {
		s.Fatalf("GoogleDrive is unexpectedly listed in %s in the container: %s", mntPath, err)
	}
}

func createFolderInDrive(ctx context.Context, filesApp *filesapp.FilesApp, keyboard *input.KeyboardEventWriter, newFolder string) error {
	// Open Google Drive
	if err := filesApp.OpenDrive(ctx); err != nil {
		return errors.Wrap(err, "failed to open Google Drive")
	}
	// Get the Files App listBox.
	filesBox, err := filesApp.Root.DescendantWithTimeout(ctx, ui.FindParams{Role: ui.RoleTypeListBox}, 15*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed getting filesBox")
	}
	defer filesBox.Release(ctx)

	// Move the focus to the file list.
	if err := filesBox.FocusAndWait(ctx, 15*time.Second); err != nil {
		return errors.Wrap(err, "failed selecting filesBox: ")
	}

	// Press ctrl+E to create a folder.
	if err := keyboard.Accel(ctx, "ctrl+E"); err != nil {
		return errors.Wrap(err, "failed to create a folder in Google Drive")
	}

	// Wait for rename text field.
	params := ui.FindParams{
		Role:  ui.RoleTypeTextField,
		State: map[ui.StateType]bool{ui.StateTypeEditable: true, ui.StateTypeFocusable: true, ui.StateTypeFocused: true},
	}
	if err := filesApp.Root.WaitUntilDescendantExists(ctx, params, 15*time.Second); err != nil {
		return errors.Wrap(err, "failed finding rename input text field")
	}

	// Name the folder with the new name.
	if err := keyboard.Type(ctx, newFolder); err != nil {
		return errors.Wrap(err, "failed to type the new name")
	}

	// Press Enter.
	if err := keyboard.Accel(ctx, "Enter"); err != nil {
		return errors.Wrap(err, "failed to rename the new folder")
	}

	// Check the newly created folder is listed Google Drive.
	if err := filesApp.WaitForFile(ctx, newFolder, 10*time.Second); err != nil {
		return errors.Wrapf(err, "failed to list the new folder %s in Drive", newFolder)
	}

	return nil
}
