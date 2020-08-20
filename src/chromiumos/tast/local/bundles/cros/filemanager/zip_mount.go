// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ZipMount,
		Desc: "Tests Files app mounting workflow",
		Contacts: []string{
			"jboulic@chromium.org",
			"fdegros@chromium.org",
			"chromeos-files-syd@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"Texts.zip", "Encrypted_AES-256.zip", "Encrypted_ZipCrypto.zip"},
	})
}

func ZipMount(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ExtraArgs("--enable-features=FilesZipMount"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	// Open the test API.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Define keyboard to perform keyboard shortcuts.
	ew, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Error creating keyboard: ", err)
	}
	defer ew.Close()

	// Open the Files App.
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Launching the Files App failed: ", err)
	}
	defer files.Root.Release(ctx)

	// Zip files names.
	const simpleZipFile = "Texts.zip"
	var encryptedZipFiles = []string{"Encrypted_AES-256.zip", "Encrypted_ZipCrypto.zip"}
	var zipFiles = []string{simpleZipFile, encryptedZipFiles[0], encryptedZipFiles[1]}

	// Open the Downloads folder.
	if err := files.OpenDownloads(ctx); err != nil {
		s.Fatal("Opening Downloads folder failed: ", err)
	}

	// Load files.
	for _, zipFile := range zipFiles {
		zipFileLocation := filepath.Join(filesapp.DownloadPath, zipFile)

		if err := fsutil.CopyFile(s.DataPath(zipFile), zipFileLocation); err != nil {
			s.Fatalf("Failed to copy zip file to %s: %s", zipFileLocation, err)
		}
		defer os.Remove(zipFileLocation)
	}

	// Order the files entries alphabetically.
	params := ui.FindParams{
		Name: "Name",
		Role: ui.RoleTypeInlineTextBox,
	}

	orderByNameButton, err := files.Root.DescendantWithTimeout(ctx, params, 5*time.Second)
	if err != nil {
		s.Fatal("Failed to find 'Name' button: ", err)
	}
	defer orderByNameButton.Release(ctx)

	if err := orderByNameButton.LeftClick(ctx); err != nil {
		s.Fatal("Failed ordering zip files by name: ", err)
	}

	testMountingSingleZipFile(ctx, s, files, ew, simpleZipFile)

	testCancellingMultiplePasswordDialog(ctx, s, files, ew, encryptedZipFiles)
}

func selectMultipleFiles(ctx context.Context, s *testing.State, files *filesapp.FilesApp, ew *input.KeyboardEventWriter, zipFiles []string) {
	// Hold ctrl during multi selection.
	if err := ew.AccelPress(ctx, "Ctrl"); err != nil {
		s.Fatal("Failed to press ctrl: ", err)
	}
	defer ew.AccelRelease(ctx, "Ctrl")

	// Select files.
	for _, zipFile := range zipFiles {
		files.SelectFile(ctx, zipFile)
	}

	// Define the number of files that we expect to select for extraction and zipping operations.
	var selectionLabel = strconv.Itoa(len(zipFiles)) + " files selected"

	// Ensure that the right number of files is selected.
	params := ui.FindParams{
		Name: selectionLabel,
		Role: ui.RoleTypeStaticText,
	}

	if err := files.Root.WaitUntilDescendantExists(ctx, params, 5*time.Second); err != nil {
		s.Fatal("Failed to find expected selection label: ", err)
	}
}

func testMountingSingleZipFile(ctx context.Context, s *testing.State, files *filesapp.FilesApp, ew *input.KeyboardEventWriter, zipFile string) {
	// Select zip file.
	if err := files.WaitForFile(ctx, zipFile, 5*time.Second); err != nil {
		s.Fatal("Waiting for test file failed: ", err)
	}

	// Click zip file and wait for Open button in top bar.
	if err := files.SelectFile(ctx, zipFile); err != nil {
		s.Fatal("Failed to mount zip file: ", err)
	}

	params := ui.FindParams{
		Name: "Open",
		Role: ui.RoleTypeButton,
	}

	open, err := files.Root.DescendantWithTimeout(ctx, params, 5*time.Second)
	if err != nil {
		s.Fatal("Failed to find Open menu item: ", err)
	}
	defer open.Release(ctx)

	// Mount zip file.
	if err := open.LeftClick(ctx); err != nil {
		s.Fatal("Mounting zip file failed: ", err)
	}

	// Ensure that the Files App is displaying the content of the mounted zip file.
	params = ui.FindParams{
		Name: "Files - " + zipFile,
		Role: ui.RoleTypeRootWebArea,
	}

	if err := files.Root.WaitUntilDescendantExists(ctx, params, 5*time.Second); err != nil {
		s.Fatal("Opening mounted zip file failed: ", err)
	}

	// Find the eject button within the appropriate tree item.
	params = ui.FindParams{
		Name: zipFile,
		Role: ui.RoleTypeTreeItem,
	}

	treeItem, err := files.Root.DescendantWithTimeout(ctx, params, 5*time.Second)
	if err != nil {
		s.Fatalf("Failed to find tree item for %s: %v", zipFile, err)
	}
	defer treeItem.Release(ctx)

	params = ui.FindParams{
		Name: "Eject device",
		Role: ui.RoleTypeButton,
	}

	ejectButton, err := treeItem.DescendantWithTimeout(ctx, params, 5*time.Second)
	if err != nil {
		s.Fatal("Failed to find eject button: ", err)
	}
	defer ejectButton.Release(ctx)

	// Click eject button to unmount the zip file.
	if err := ejectButton.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click eject button: ", err)
	}

	// Checks that the tree item corresponding to the previously mounted zip file was removed.
	params = ui.FindParams{
		Name: zipFile,
		Role: ui.RoleTypeTreeItem,
	}

	if err = files.Root.WaitUntilDescendantGone(ctx, params, 5*time.Second); err != nil {
		s.Fatalf("%s is still listed in Files app: %v", zipFile, err)
	}
}

func testCancellingMultiplePasswordDialog(ctx context.Context, s *testing.State, files *filesapp.FilesApp, ew *input.KeyboardEventWriter, encryptedZipFiles []string) {
	// Open the Downloads folder.
	if err := files.OpenDownloads(ctx); err != nil {
		s.Fatal("Opening Downloads folder failed: ", err)
	}

	// Select the 2 encrypted zip files.
	selectMultipleFiles(ctx, s, files, ew, encryptedZipFiles)

	// Press Enter.
	if err := ew.Accel(ctx, "Enter"); err != nil {
		s.Fatal("Pressing Enter failed: ", err)
	}

	// Get password dialog.
	params := ui.FindParams{
		Name: "Password",
		Role: ui.RoleTypeDialog,
	}

	passwordDialog, err := files.Root.DescendantWithTimeout(ctx, params, 5*time.Second)
	if err != nil {
		s.Fatal("Failed to find password dialog: ", err)
	}
	defer passwordDialog.Release(ctx)

	// The filename displayed on the password dialog should be the one of the first encrypted zip archive.
	params = ui.FindParams{
		Name: encryptedZipFiles[0],
		Role: ui.RoleTypeInlineTextBox,
	}

	if err := passwordDialog.WaitUntilDescendantExists(ctx, params, 5*time.Second); err != nil {
		s.Fatalf("Unable to find expected %q in password dialog: %v", encryptedZipFiles[0], err)
	}

	// Press escape.
	if err := ew.Accel(ctx, "Esc"); err != nil {
		s.Fatal("Pressing Escape failed: ", err)
	}

	// Wait until the password dialog is active for the second encrypted zip archive.
	testing.Poll(ctx, func(ctx context.Context) error {
		params = ui.FindParams{
			Name: "Password",
			Role: ui.RoleTypeDialog,
		}

		passwordDialog, err = files.Root.DescendantWithTimeout(ctx, params, 5*time.Second)
		if err != nil {
			s.Fatal("Failed to find password dialog: ", err)
		}
		defer passwordDialog.Release(ctx)

		// The filename displayed on password dialog should be updated to the second encrypted zip archive.
		params = ui.FindParams{
			Name: encryptedZipFiles[1],
			Role: ui.RoleTypeInlineTextBox,
		}

		if err := passwordDialog.WaitUntilDescendantExists(ctx, params, 5*time.Second); err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: 15 * time.Second})

	// Click cancel.
	params = ui.FindParams{
		Name: "Cancel",
		Role: ui.RoleTypeButton,
	}

	cancel, err := files.Root.DescendantWithTimeout(ctx, params, 5*time.Second)
	if err != nil {
		s.Fatal("Failed to find password dialog cancel button: ", err)
	}
	defer cancel.Release(ctx)

	if err := cancel.LeftClick(ctx); err != nil {
		s.Fatal("Cancelling password dialog failed: ", err)
	}

	// Checks that the password dialog is not displayed anymore.
	params = ui.FindParams{
		Name: "Password",
		Role: ui.RoleTypeDialog,
	}

	if err = files.Root.WaitUntilDescendantGone(ctx, params, 5*time.Second); err != nil {
		s.Fatal("The password dialog is still displayed: ", err)
	}
}
