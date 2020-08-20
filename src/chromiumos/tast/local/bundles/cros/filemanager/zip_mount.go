// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
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
		Params: []testing.Param{{
			Name: "mount_single",
			Val:  "mountSingle",
		}, {
			Name: "cancel_multiple",
			Val:  "cancelMultiple",
		}, {
			Name: "mount_multiple",
			Val:  "mountMultiple",
		}},
	})
}

func ZipMount(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ExtraArgs("--enable-features=FilesZipMount"))
	if err != nil {
		s.Fatal("Cannot start Chrome: ", err)
	}

	// ZIP file names.
	const simpleZipFile = "Texts.zip"
	var encryptedZipFiles = []string{"Encrypted_AES-256.zip", "Encrypted_ZipCrypto.zip"}
	var zipFiles = append(encryptedZipFiles, simpleZipFile)

	// Load ZIP files.
	for _, zipFile := range zipFiles {
		zipFileLocation := filepath.Join(filesapp.DownloadPath, zipFile)

		if err := fsutil.CopyFile(s.DataPath(zipFile), zipFileLocation); err != nil {
			s.Fatalf("Cannot copy ZIP file to %s: %s", zipFileLocation, err)
		}
		defer os.Remove(zipFileLocation)
	}

	// Open the test API.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Cannot create test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Define keyboard to perform keyboard shortcuts.
	ew, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Cannot create keyboard: ", err)
	}
	defer ew.Close()

	// Open the Files App.
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Cannot launch the Files App: ", err)
	}
	defer files.Root.Release(ctx)

	// Open the Downloads folder.
	if err := files.OpenDownloads(ctx); err != nil {
		s.Fatal("Cannot open Downloads folder: ", err)
	}

	// Find and click the 'Name' button to order the file entries alphabetically.
	// Polling is used since the 'Name' button is not always clickable at the time the corresponding node is found.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		params := ui.FindParams{
			Name: "Name",
			Role: ui.RoleTypeButton,
		}

		orderByNameButton, err := files.Root.Descendant(ctx, params)
		if err != nil {
			return errors.New("name button still not found")
		}
		defer orderByNameButton.Release(ctx)

		if err := orderByNameButton.LeftClick(ctx); err != nil {
			return errors.New("still unable to click name button to sort files alphabetically")
		}

		return nil
	}, &testing.PollOptions{Timeout: 15 * time.Second}); err != nil {
		s.Fatal("Cannot click 'Name' button: ", err)
	}

	// Wait until the ZIP files are correctly ordered in the list box.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		params := ui.FindParams{
			Role:  ui.RoleTypeListBox,
			State: map[ui.StateType]bool{ui.StateTypeFocusable: true, ui.StateTypeMultiselectable: true, ui.StateTypeVertical: true},
		}

		listBox, err := files.Root.Descendant(ctx, params)
		if err != nil {
			return testing.PollBreak(err)
		}
		defer listBox.Release(ctx)

		nodes, err := listBox.Descendants(ctx, ui.FindParams{Role: ui.RoleTypeListBoxOption})
		if err != nil {
			return testing.PollBreak(err)
		}
		defer nodes.Release(ctx)

		// The names of the descendant nodes should be ordered alphabetically.
		for i, node := range nodes {
			if node.Name != zipFiles[i] {
				return errors.New("the files are still not ordered properly")
			}
		}

		return nil
	}, &testing.PollOptions{Timeout: 15 * time.Second}); err != nil {
		s.Fatal("Cannot sort ZIP files properly in the Files app list box: ", err)
	}

	switch s.Param().(string) {
	case "mountSingle":
		testMountingSingleZipFile(ctx, s, files, simpleZipFile)
	case "cancelMultiple":
		testCancelingMultiplePasswordDialogs(ctx, s, files, ew, encryptedZipFiles)
	case "mountMultiple":
		testMountingMultipleZipFiles(ctx, s, files, ew, zipFiles)
	default:
		s.Fatal("Unexpected test param: ", s.Param())
	}
}

// selectMultipleFiles selects multiple files while pressing 'Ctrl'.
func selectMultipleFiles(ctx context.Context, s *testing.State, files *filesapp.FilesApp, ew *input.KeyboardEventWriter, zipFiles []string) {
	// Hold Ctrl during multi selection.
	if err := ew.AccelPress(ctx, "Ctrl"); err != nil {
		s.Fatal("Cannot press Ctrl: ", err)
	}
	defer ew.AccelRelease(ctx, "Ctrl")

	// Select files.
	for _, zipFile := range zipFiles {
		if err := files.SelectFile(ctx, zipFile); err != nil {
			s.Fatalf("Cannot select %s : %v", zipFile, err)
		}
	}

	// Define the label associated to the number of files we are selecting.
	var selectionLabel = fmt.Sprintf("%d files selected", len(zipFiles))

	// Ensure that the right number of files is selected.
	params := ui.FindParams{
		Name: selectionLabel,
		Role: ui.RoleTypeStaticText,
	}

	if err := files.Root.WaitUntilDescendantExists(ctx, params, 5*time.Second); err != nil {
		s.Fatal("Cannot find expected selection label: ", err)
	}
}

// waitUntilPasswordDialogExists waits for the password dialog to display for a specific encrypted ZIP file.
func waitUntilPasswordDialogExists(ctx context.Context, files *filesapp.FilesApp, fileName string) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		// Get reference to the current password dialog.
		params := ui.FindParams{
			Name: "Password",
			Role: ui.RoleTypeDialog,
		}

		passwordDialog, err := files.Root.Descendant(ctx, params)
		if err != nil {
			return errors.New("password dialog still not found")
		}
		defer passwordDialog.Release(ctx)

		// Look for expected file name label within the current dialog.
		params = ui.FindParams{
			Name: fileName,
			Role: ui.RoleTypeStaticText,
		}
		exists, err := passwordDialog.DescendantExists(ctx, params)
		if err != nil || !exists {
			return errors.New("expected file name label still not found")
		}

		return nil
	}, &testing.PollOptions{Timeout: 15 * time.Second})
}

// checkAndUnmountZipFile checks that a given ZIP file is correctly mounted and click the 'eject' button to unmount it.
func checkAndUnmountZipFile(ctx context.Context, s *testing.State, files *filesapp.FilesApp, zipFile string) {
	// Find and open the mounted ZIP file.
	params := ui.FindParams{
		Name: zipFile,
		Role: ui.RoleTypeTreeItem,
	}

	treeItem, err := files.Root.DescendantWithTimeout(ctx, params, 5*time.Second)
	if err != nil {
		s.Fatalf("Cannot find tree item for %s: %v", zipFile, err)
	}
	defer treeItem.Release(ctx)

	if err := treeItem.LeftClick(ctx); err != nil {
		s.Fatal("Cannot open mounted ZIP file: ", err)
	}

	// Ensure that the Files App is displaying the content of the mounted ZIP file.
	params = ui.FindParams{
		Name: "Files - " + zipFile,
		Role: ui.RoleTypeRootWebArea,
	}

	if err := files.Root.WaitUntilDescendantExists(ctx, params, 5*time.Second); err != nil {
		s.Fatal("Cannot see content of mounted ZIP file: ", err)
	}

	// The test ZIP files are all expected to contain a single "Texts" folder.
	var zipContentDirectoryLabel = "Texts"

	// Check content of mounted ZIP file.
	params = ui.FindParams{
		Name: zipContentDirectoryLabel,
		Role: ui.RoleTypeListBoxOption,
	}

	if err := files.Root.WaitUntilDescendantExists(ctx, params, 5*time.Second); err != nil {
		s.Fatalf("Cannot see directory %s in mounted ZIP file: %v", zipContentDirectoryLabel, err)
	}

	// Find the eject button within the appropriate tree item.
	params = ui.FindParams{
		Name: "Eject device",
		Role: ui.RoleTypeButton,
	}

	ejectButton, err := treeItem.DescendantWithTimeout(ctx, params, 5*time.Second)
	if err != nil {
		s.Fatal("Cannot find eject button: ", err)
	}
	defer ejectButton.Release(ctx)

	// Click eject button to unmount the ZIP file.
	if err := ejectButton.LeftClick(ctx); err != nil {
		s.Fatal("Cannot click eject button: ", err)
	}

	// Check that the tree item corresponding to the previously mounted ZIP file was removed.
	params = ui.FindParams{
		Name: zipFile,
		Role: ui.RoleTypeTreeItem,
	}

	if err = files.Root.WaitUntilDescendantGone(ctx, params, 5*time.Second); err != nil {
		s.Fatalf("%s is still mounted: %v", zipFile, err)
	}
}

func testMountingSingleZipFile(ctx context.Context, s *testing.State, files *filesapp.FilesApp, zipFile string) {
	// Select ZIP file.
	if err := files.WaitForFile(ctx, zipFile, 5*time.Second); err != nil {
		s.Fatal("Cannot wait for test ZIP file: ", err)
	}

	if err := files.SelectFile(ctx, zipFile); err != nil {
		s.Fatal("Cannot select ZIP file: ", err)
	}

	// Wait for Open button in the top bar.
	params := ui.FindParams{
		Name: "Open",
		Role: ui.RoleTypeButton,
	}

	open, err := files.Root.DescendantWithTimeout(ctx, params, 5*time.Second)
	if err != nil {
		s.Fatal("Cannot find Open menu item: ", err)
	}
	defer open.Release(ctx)

	// Mount ZIP file.
	if err := open.LeftClick(ctx); err != nil {
		s.Fatal("Cannot mount ZIP file: ", err)
	}

	checkAndUnmountZipFile(ctx, s, files, zipFile)
}

func testCancelingMultiplePasswordDialogs(ctx context.Context, s *testing.State, files *filesapp.FilesApp, ew *input.KeyboardEventWriter, encryptedZipFiles []string) {
	// Select the 2 encrypted ZIP files.
	selectMultipleFiles(ctx, s, files, ew, encryptedZipFiles)

	// Press Enter.
	if err := ew.Accel(ctx, "Enter"); err != nil {
		s.Fatal("Cannot press 'Enter': ", err)
	}

	// Wait until the password dialog is active for the first encrypted ZIP archive.
	if err := waitUntilPasswordDialogExists(ctx, files, encryptedZipFiles[0]); err != nil {
		s.Fatalf("Cannot find password dialog for %s : %v", encryptedZipFiles[0], err)
	}

	// Press Esc.
	if err := ew.Accel(ctx, "Esc"); err != nil {
		s.Fatal("Cannot press 'Esc': ", err)
	}

	// Wait until the password dialog is active for the second encrypted ZIP archive.
	if err := waitUntilPasswordDialogExists(ctx, files, encryptedZipFiles[1]); err != nil {
		s.Fatalf("Cannot find password dialog for %s : %v", encryptedZipFiles[1], err)
	}

	// Click the 'Cancel' button.
	params := ui.FindParams{
		Name: "Cancel",
		Role: ui.RoleTypeButton,
	}

	cancel, err := files.Root.DescendantWithTimeout(ctx, params, 5*time.Second)
	if err != nil {
		s.Fatal("Cannot find password dialog cancel button: ", err)
	}
	defer cancel.Release(ctx)

	if err := cancel.LeftClick(ctx); err != nil {
		s.Fatal("Cannot cancel password dialog: ", err)
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

func testMountingMultipleZipFiles(ctx context.Context, s *testing.State, files *filesapp.FilesApp, ew *input.KeyboardEventWriter, zipFiles []string) {
	// Select the 3 encrypted zip files
	selectMultipleFiles(ctx, s, files, ew, zipFiles)

	// Press Enter.
	if err := ew.Accel(ctx, "Enter"); err != nil {
		s.Fatal("Cannot press 'Enter': ", err)
	}

	// Wait until the password dialog is active for the first encrypted ZIP archive.
	if err := waitUntilPasswordDialogExists(ctx, files, zipFiles[0]); err != nil {
		s.Fatalf("Cannot find password dialog for %s : %v", zipFiles[0], err)
	}

	// Enter wrong password.
	params := ui.FindParams{
		Role:  ui.RoleTypeTextField,
		State: map[ui.StateType]bool{ui.StateTypeEditable: true, ui.StateTypeFocusable: true, ui.StateTypeProtected: true, ui.StateTypeFocused: true},
	}

	if err := files.Root.WaitUntilDescendantExists(ctx, params, 15*time.Second); err != nil {
		s.Fatal("Cannot find password input field: ", err)
	}

	if err := ew.Type(ctx, "wrongPassword"); err != nil {
		s.Fatal("Cannot enter 'wrongPassword': ", err)
	}

	// Validate password.
	if err := ew.Accel(ctx, "Enter"); err != nil {
		s.Fatal("Cannot validate password by pressing 'Enter': ", err)
	}

	// Check that the password dialog is still active for the first encrypted ZIP archive.
	if err := waitUntilPasswordDialogExists(ctx, files, zipFiles[0]); err != nil {
		s.Fatalf("Cannot find password dialog for %s : %v", zipFiles[0], err)
	}

	// Check that the password field is prefilled with the last entered password.
	params = ui.FindParams{
		Name:  "•••••••••••••",
		Role:  ui.RoleTypeStaticText,
		State: map[ui.StateType]bool{ui.StateTypeEditable: true},
	}

	if err := files.Root.WaitUntilDescendantExists(ctx, params, 15*time.Second); err != nil {
		s.Fatal("Cannot find last entered password: ", err)
	}

	// Press Backspace.
	if err := ew.Accel(ctx, "Backspace"); err != nil {
		s.Fatal("Cannot press 'Backspace': ", err)
	}

	// Enter right password.
	if err := ew.Type(ctx, "password"); err != nil {
		s.Fatal("Cannot enter 'password': ", err)
	}

	// Validate password.
	if err := ew.Accel(ctx, "Enter"); err != nil {
		s.Fatal("Cannot validate the name of the new directory: ", err)
	}

	// Check that the password dialog is active for the second encrypted ZIP archive.
	if err := waitUntilPasswordDialogExists(ctx, files, zipFiles[1]); err != nil {
		s.Fatalf("Cannot find password dialog for %s : %v", zipFiles[1], err)
	}

	// Press Backspace.
	if err := ew.Accel(ctx, "Backspace"); err != nil {
		s.Fatal("Cannot press 'Backspace': ", err)
	}

	// Enter right password.
	if err := ew.Type(ctx, "password"); err != nil {
		s.Fatal("Cannot enter 'password': ", err)
	}

	// Click Unlock.
	params = ui.FindParams{
		Name: "Unlock",
		Role: ui.RoleTypeButton,
	}

	unlock, err := files.Root.DescendantWithTimeout(ctx, params, 5*time.Second)
	if err != nil {
		s.Fatal("Cannot find password dialog cancel button: ", err)
	}
	defer unlock.Release(ctx)

	if err := unlock.LeftClick(ctx); err != nil {
		s.Fatal("Cannot validate password with the Unlock button: ", err)
	}

	// Checks that the password dialog is not displayed anymore.
	params = ui.FindParams{
		Name: "Password",
		Role: ui.RoleTypeDialog,
	}

	if err = files.Root.WaitUntilDescendantGone(ctx, params, 5*time.Second); err != nil {
		s.Fatal("The password dialog is still displayed: ", err)
	}

	// Check that the 3 zip files have been mounted correctly and unmount them.
	for _, zipFile := range zipFiles {
		checkAndUnmountZipFile(ctx, s, files, zipFile)
	}
}
