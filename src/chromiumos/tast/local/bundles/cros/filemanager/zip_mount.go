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
	})
}

func ZipMount(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ExtraArgs("--enable-features=FilesZipMount"))
	if err != nil {
		s.Fatal("Cannot start Chrome: ", err)
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

	// ZIP file names.
	const simpleZipFile = "Texts.zip"
	var encryptedZipFiles = []string{"Encrypted_AES-256.zip", "Encrypted_ZipCrypto.zip"}
	var zipFiles = []string{simpleZipFile, encryptedZipFiles[0], encryptedZipFiles[1]}

	// Open the Downloads folder.
	if err := files.OpenDownloads(ctx); err != nil {
		s.Fatal("Cannot open Downloads folder: ", err)
	}

	// Load files.
	for _, zipFile := range zipFiles {
		zipFileLocation := filepath.Join(filesapp.DownloadPath, zipFile)

		if err := fsutil.CopyFile(s.DataPath(zipFile), zipFileLocation); err != nil {
			s.Fatalf("Cannot copy ZIP file to %s: %s", zipFileLocation, err)
		}
		defer os.Remove(zipFileLocation)
	}

	// Wait for location change events to be propagated.
	if err := ui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
		s.Fatal("Cannot wait for location change completed: ", err)
	}

	// Order the file entries alphabetically.
	params := ui.FindParams{
		Name: "Name",
		Role: ui.RoleTypeButton,
	}

	orderByNameButton, err := files.Root.DescendantWithTimeout(ctx, params, 5*time.Second)
	if err != nil {
		s.Fatal("Cannot find 'Name' button: ", err)
	}
	defer orderByNameButton.Release(ctx)

	if err := orderByNameButton.LeftClick(ctx); err != nil {
		s.Fatal("Cannot click 'Name' button to order files alphabetically: ", err)
	}

	// Wait until the ZIP files are correctly ordered in the list box.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		params := ui.FindParams{
			Role:  ui.RoleTypeListBox,
			State: map[ui.StateType]bool{ui.StateTypeFocusable: true, ui.StateTypeMultiselectable: true, ui.StateTypeVertical: true},
		}

		listBox, err := files.Root.DescendantWithTimeout(ctx, params, 5*time.Second)
		if err != nil {
			return testing.PollBreak(err)
		}
		defer listBox.Release(ctx)

		nodes, err := listBox.Descendants(ctx, ui.FindParams{Role: ui.RoleTypeListBoxOption})
		if err != nil {
			s.Fatal("Cannot retrieve listBox descendants: ", err)
		}

		// The names of the descendant nodes should be ordered alphabetically.
		var expectedZipFiles = []string{"Encrypted_AES-256.zip", "Encrypted_ZipCrypto.zip", "Texts.zip"}
		var sorted = true
		for i, node := range nodes {
			if node.Name != expectedZipFiles[i] {
				sorted = false
			}
			node.Release(ctx)
		}
		if !sorted {
			return errors.New("The files are still not ordered properly")
		}
		return nil
	}, &testing.PollOptions{Timeout: 15 * time.Second}); err != nil {
		s.Fatal("Cannot sort ZIP files properly in the Files app list box: ", err)
	}

	testMountingSingleZipFile(ctx, s, files, simpleZipFile)

	testCancelingMultiplePasswordDialog(ctx, s, files, ew, encryptedZipFiles)
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
		files.SelectFile(ctx, zipFile)
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
func waitUntilPasswordDialogExists(ctx context.Context, s *testing.State, files *filesapp.FilesApp, fileName string) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		// Get reference to the current password dialog.
		params := ui.FindParams{
			Name: "Password",
			Role: ui.RoleTypeDialog,
		}

		passwordDialog, err := files.Root.DescendantWithTimeout(ctx, params, 5*time.Second)
		if err != nil {
			return testing.PollBreak(err)
		}
		defer passwordDialog.Release(ctx)

		// Look for expected file name label within the current dialog.
		params = ui.FindParams{
			Name: fileName,
			Role: ui.RoleTypeStaticText,
		}
		if err := passwordDialog.WaitUntilDescendantExists(ctx, params, time.Second); err != nil {
			return errors.New("expected file name label still not found")
		}

		return nil
	}, &testing.PollOptions{Timeout: 15 * time.Second})
}

// unmountZipFile checks that a given ZIP file is correctly mounted and click the 'eject' button to unmount it.
func unmountZipFile(ctx context.Context, s *testing.State, files *filesapp.FilesApp, zipFile string) {
	// Find the eject button within the appropriate tree item.
	params := ui.FindParams{
		Name: zipFile,
		Role: ui.RoleTypeTreeItem,
	}

	treeItem, err := files.Root.DescendantWithTimeout(ctx, params, 5*time.Second)
	if err != nil {
		s.Fatalf("Cannot find tree item for %s: %v", zipFile, err)
	}
	defer treeItem.Release(ctx)

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

	// Ensure that the Files App is displaying the content of the mounted ZIP file.
	params = ui.FindParams{
		Name: "Files - " + zipFile,
		Role: ui.RoleTypeRootWebArea,
	}

	if err := files.Root.WaitUntilDescendantExists(ctx, params, 5*time.Second); err != nil {
		s.Fatal("Cannot see content of mounted ZIP file: ", err)
	}
}

func testCancelingMultiplePasswordDialog(ctx context.Context, s *testing.State, files *filesapp.FilesApp, ew *input.KeyboardEventWriter, encryptedZipFiles []string) {
	// Open the Downloads folder.
	if err := files.OpenDownloads(ctx); err != nil {
		s.Fatal("Cannot open Downloads folder: ", err)
	}

	// Select the 2 encrypted ZIP files.
	selectMultipleFiles(ctx, s, files, ew, encryptedZipFiles)

	// Press Enter.
	if err := ew.Accel(ctx, "Enter"); err != nil {
		s.Fatal("Cannot press 'Enter': ", err)
	}

	// Wait until the password dialog is active for the first encrypted ZIP archive.
	if err := waitUntilPasswordDialogExists(ctx, s, files, encryptedZipFiles[0]); err != nil {
		s.Fatalf("Cannot find password dialog for %s : %v", encryptedZipFiles[0], err)
	}

	// Press Esc.
	if err := ew.Accel(ctx, "Esc"); err != nil {
		s.Fatal("Cannot press 'Esc': ", err)
	}

	// Wait until the password dialog is active for the second encrypted ZIP archive.
	if err := waitUntilPasswordDialogExists(ctx, s, files, encryptedZipFiles[1]); err != nil {
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
