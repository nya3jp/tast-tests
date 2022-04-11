// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// TestFunc contains the contents of the test itself.
type testFunc func(ctx context.Context, s *testing.State, files *filesapp.FilesApp, zipFiles []string)

// TestEntry contains the function used in the test along with the names of the ZIP files the test is using.
type testEntry struct {
	TestCase testFunc
	ZipFiles []string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ZipMount,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests Files app mounting workflow",
		Contacts: []string{
			"jboulic@chromium.org",
			"fdegros@chromium.org",
			"chromeos-files-syd@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Data: []string{
			"Encrypted_AES-256.zip",
			"Encrypted_ZipCrypto.zip",
			"Texts.7z",
			"Texts.rar",
			"Texts.zip",
		},
		Params: []testing.Param{{
			Name: "mount_single_7z",
			Val: testEntry{
				TestCase: testMountingSingleZipFile,
				ZipFiles: []string{"Texts.7z"},
			},
		}, {
			Name: "mount_single_rar",
			Val: testEntry{
				TestCase: testMountingSingleZipFile,
				ZipFiles: []string{"Texts.rar"},
			},
		}, {
			Name: "mount_single_zip",
			Val: testEntry{
				TestCase: testMountingSingleZipFile,
				ZipFiles: []string{"Texts.zip"},
			},
		}, {
			Name: "cancel_multiple",
			Val: testEntry{
				TestCase: testCancelingMultiplePasswordDialogs,
				ZipFiles: []string{"Encrypted_AES-256.zip", "Encrypted_ZipCrypto.zip"},
			},
		}, {
			Name: "mount_multiple",
			Val: testEntry{
				TestCase: testMountingMultipleZipFiles,
				ZipFiles: []string{"Encrypted_AES-256.zip", "Encrypted_ZipCrypto.zip", "Texts.zip"},
			},
		}},
	})
}

func ZipMount(ctx context.Context, s *testing.State) {
	testParams := s.Param().(testEntry)
	zipFiles := testParams.ZipFiles

	// TODO(nigeltao): remove "FilesArchivemount" after it gets flipped to
	// enabled-by-default (scheduled for M94) and before the feature flag
	// expires (scheduled for M100). crbug.com/1216245
	cr, err := chrome.New(ctx, chrome.EnableFeatures("FilesArchivemount"))
	if err != nil {
		s.Fatal("Cannot start Chrome: ", err)
	}
	defer cr.Close(ctx)

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

	// Open the Files App.
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Cannot launch the Files App: ", err)
	}

	// Open the Downloads folder.
	if err := files.OpenDownloads()(ctx); err != nil {
		s.Fatal("Cannot open Downloads folder: ", err)
	}

	// Wait for the ZIP files to load in the file list.
	for _, zipFile := range zipFiles {
		if err := files.WaitForFile(zipFile)(ctx); err != nil {
			s.Fatalf("Cannot find %s: %s", zipFile, err)
		}
	}

	testParams.TestCase(ctx, s, files, zipFiles)
}

// waitUntilPasswordDialogExists waits until the password dialog is displayed and returns the expected ZIP file name to which it is associated.
func waitUntilPasswordDialogExists(ctx context.Context, files *filesapp.FilesApp, processedZipFiles map[string]bool) (string, error) {
	// Zip file name to be returned, to which the password dialog is associated.
	zipFileName := ""

	// Wait until the password dialog is displayed for one of the expected ZIP files.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		passwordDialog := nodewith.Role(role.Dialog)

		// Wait for a dialog to open.
		if err := files.WithTimeout(5 * time.Second).WaitUntilExists(passwordDialog)(ctx); err != nil {
			return errors.Wrap(err, "cannot find dialog")
		}

		// Look for one of the expected ZIP file names within the dialog.
		nodes, err := files.NodesInfo(ctx, nodewith.Role(role.StaticText).Ancestor(passwordDialog))
		if err != nil || len(nodes) == 0 {
			return errors.New("cannot find static text in dialog")
		}

		// Match the text found in the dialog with the zipFiles for which we still expect a password dialog.
		for _, n := range nodes {
			if _, exists := processedZipFiles[n.Name]; exists {
				zipFileName = n.Name
				return nil
			}
		}

		return errors.New("cannot find expected ZIP file name in dialog")
	}, &testing.PollOptions{Timeout: 15 * time.Second}); err != nil {
		return "", err
	}

	return zipFileName, nil
}

// checkAndUnmountZipFile checks that a given ZIP file is correctly mounted and click the 'eject' button to unmount it.
func checkAndUnmountZipFile(ctx context.Context, s *testing.State, files *filesapp.FilesApp, zipFile string) {
	// Find and open the mounted ZIP file.
	zipFileNode := nodewith.Name(zipFile).Role(role.TreeItem)
	if err := uiauto.Combine("find and click tree item",
		files.WithTimeout(5*time.Second).WaitUntilExists(zipFileNode),
		files.LeftClick(zipFileNode),
	)(ctx); err != nil {
		s.Fatal("Cannot find and open the mounted ZIP file: ", err)
	}

	// Ensure that the Files App is displaying the content of the mounted ZIP file.
	rootWebArea := nodewith.Name("Files - " + zipFile).Role(role.RootWebArea)
	if err := files.WithTimeout(5 * time.Second).WaitUntilExists(rootWebArea)(ctx); err != nil {
		s.Fatal("Cannot see content of mounted ZIP file: ", err)
	}

	// The test ZIP files are all expected to contain a single "Texts" folder.
	var zipContentDirectoryLabel = "Texts"

	// Check content of mounted ZIP file.
	label := nodewith.Name(zipContentDirectoryLabel).Role(role.ListBoxOption)
	if err := files.WithTimeout(5 * time.Second).WaitUntilExists(label)(ctx); err != nil {
		s.Fatalf("Cannot see directory %s in mounted ZIP file: %v", zipContentDirectoryLabel, err)
	}

	// Find the eject button within the appropriate tree item.
	ejectButton := nodewith.Name("Eject device").Role(role.Button).Ancestor(zipFileNode)
	if err := uiauto.Combine("find and click eject button - "+zipFile,
		files.WithTimeout(5*time.Second).WaitUntilExists(ejectButton),
		files.LeftClick(ejectButton),
	)(ctx); err != nil {
		s.Fatal("Cannot find the eject button within the appropriate tree item: ", err)
	}

	// Check that the tree item corresponding to the previously mounted ZIP file was removed.
	if err := files.WithTimeout(5 * time.Second).WaitUntilGone(zipFileNode)(ctx); err != nil {
		s.Fatalf("%s is still mounted: %v", zipFile, err)
	}
}

func testMountingSingleZipFile(ctx context.Context, s *testing.State, files *filesapp.FilesApp, zipFiles []string) {
	if len(zipFiles) != 1 {
		s.Fatal("Unexpected length for zipFiles")
	}
	zipFile := zipFiles[0]

	// Select ZIP file.
	if err := uiauto.Combine("wait for test ZIP file and select",
		files.WithTimeout(5*time.Second).WaitForFile(zipFile),
		files.SelectFile(zipFile),
	)(ctx); err != nil {
		s.Fatal("Cannot select ZIP file: ", err)
	}

	// Wait for Open button in the top bar.
	open := nodewith.Name("Open").Role(role.Button)
	if err := uiauto.Combine("find and click Open menu item",
		files.WithTimeout(5*time.Second).WaitUntilExists(open),
		files.LeftClick(open),
	)(ctx); err != nil {
		s.Fatal("Cannot unmount Zip file: ", err)
	}

	checkAndUnmountZipFile(ctx, s, files, zipFile)
}

func testCancelingMultiplePasswordDialogs(ctx context.Context, s *testing.State, files *filesapp.FilesApp, zipFiles []string) {
	// Define keyboard to perform keyboard shortcuts.
	ew, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Cannot create keyboard: ", err)
	}
	defer ew.Close()

	// Select the 2 encrypted ZIP files.
	if err := files.SelectMultipleFiles(ew, zipFiles...)(ctx); err != nil {
		s.Fatal("Cannot perform multi-selection on the encrypted files: ", err)
	}

	// Press Enter.
	if err := ew.Accel(ctx, "Enter"); err != nil {
		s.Fatal("Cannot press 'Enter': ", err)
	}

	// Define a map of zipFileNames for which we still expect a password dialog.
	processedZipFiles := make(map[string]bool)
	for _, zipFile := range zipFiles {
		processedZipFiles[zipFile] = true
	}

	// Wait until the password dialog is active for one of the encrypted ZIP archives.
	firstZipFileName, err1 := waitUntilPasswordDialogExists(ctx, files, processedZipFiles)
	if err1 != nil {
		s.Fatal("Cannot find password dialog: ", err1)
	}

	// We don't expect another password dialog for this particular file.
	delete(processedZipFiles, firstZipFileName)

	// Press Esc.
	if err := ew.Accel(ctx, "Esc"); err != nil {
		s.Fatal("Cannot press 'Esc': ", err)
	}

	// Wait until the password dialog is active for the second encrypted ZIP archive.
	if _, err2 := waitUntilPasswordDialogExists(ctx, files, processedZipFiles); err2 != nil {
		s.Fatal("Cannot find password dialog: ", err2)
	}

	// Click the 'Cancel' button.
	cancel := nodewith.Name("Cancel").Role(role.Button)
	if err := uiauto.Combine("find and click password dialog cancel button",
		files.WithTimeout(5*time.Second).WaitUntilExists(cancel),
		files.LeftClick(cancel),
	)(ctx); err != nil {
		s.Fatal("Cannot click the 'Cancel' button: ", err)
	}

	// Checks that the password dialog is not displayed anymore.
	passwordDialog := nodewith.Name("Password").Role(role.Dialog)
	if err = files.WithTimeout(5 * time.Second).WaitUntilGone(passwordDialog)(ctx); err != nil {
		s.Fatal("The password dialog is still displayed: ", err)
	}
}

func testMountingMultipleZipFiles(ctx context.Context, s *testing.State, files *filesapp.FilesApp, zipFiles []string) {
	// Define keyboard to perform keyboard shortcuts.
	ew, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Cannot create keyboard: ", err)
	}
	defer ew.Close()

	// Define a map of zipFileNames for which we still expect a password dialog.
	processedZipFiles := make(map[string]bool)
	for _, zipFile := range zipFiles {
		processedZipFiles[zipFile] = true
	}

	// Select the 3 ZIP files. 2 of them are encrypted.
	if err := files.SelectMultipleFiles(ew, zipFiles...)(ctx); err != nil {
		s.Fatal("Cannot perform multi-selection on the encrypted files: ", err)
	}

	// Press Enter.
	if err := ew.Accel(ctx, "Enter"); err != nil {
		s.Fatal("Cannot press 'Enter': ", err)
	}

	// Wait until the password dialog is active for one of the encrypted ZIP archives.
	firstZipFileName, err1 := waitUntilPasswordDialogExists(ctx, files, processedZipFiles)
	if err1 != nil {
		s.Fatal("Cannot find password dialog: ", err1)
	}

	// Enter wrong password.
	node := nodewith.Role(role.TextField).Editable().Focusable().Protected().Focused()
	if err := files.WithTimeout(15 * time.Second).WaitUntilExists(node)(ctx); err != nil {
		s.Fatal("Cannot find password input field: ", err)
	}

	if err := ew.Type(ctx, "wrongPassword"); err != nil {
		s.Fatal("Cannot enter 'wrongPassword': ", err)
	}

	// Validate password.
	if err := ew.Accel(ctx, "Enter"); err != nil {
		s.Fatal("Cannot validate password by pressing 'Enter': ", err)
	}

	// Check that the password dialog is still active.
	firstZipFileName2, err := waitUntilPasswordDialogExists(ctx, files, processedZipFiles)
	if err != nil {
		s.Fatal("Cannot find password dialog: ", err)
	}
	if firstZipFileName != firstZipFileName2 {
		s.Fatalf("Password dialog unexpectedly changed: %s to %s", firstZipFileName, firstZipFileName2)
	}

	// Check that "Invalid password" is displayed on the UI.
	invalidPassword := nodewith.Name("Invalid password").Role(role.StaticText)
	if err := files.WithTimeout(15 * time.Second).WaitUntilExists(invalidPassword)(ctx); err != nil {
		s.Fatal("Cannot find 'Invalid password': ", err)
	}

	// Enter right password.
	if err := ew.Type(ctx, "password"); err != nil {
		s.Fatal("Cannot enter 'password': ", err)
	}

	// Validate password.
	if err := ew.Accel(ctx, "Enter"); err != nil {
		s.Fatal("Cannot validate the name of the new directory: ", err)
	}

	// We don't expect another password dialog for this particular file.
	delete(processedZipFiles, firstZipFileName)

	// Check that the password dialog is active for the second encrypted ZIP archive.
	if _, err := waitUntilPasswordDialogExists(ctx, files, processedZipFiles); err != nil {
		s.Fatal("Cannot find password dialog: ", err)
	}

	// Enter right password.
	if err := ew.Type(ctx, "password"); err != nil {
		s.Fatal("Cannot enter 'password': ", err)
	}

	// Click Unlock.
	unlock := nodewith.Name("Unlock").Role(role.Button)
	if err := uiauto.Combine("find password dialog cancel button and validate password",
		files.WithTimeout(5*time.Second).WaitUntilExists(unlock),
		files.LeftClick(unlock),
	)(ctx); err != nil {
		s.Fatal("Cannot find password dialog cancel button: ", err)
	}

	// Checks that the password dialog is not displayed anymore.
	password := nodewith.Name("Password").Role(role.Dialog)
	if err = files.WithTimeout(5 * time.Second).WaitUntilGone(password)(ctx); err != nil {
		s.Fatal("The password dialog is still displayed: ", err)
	}

	// Check that the 3 zip files have been mounted correctly and unmount them.
	for _, zipFile := range zipFiles {
		checkAndUnmountZipFile(ctx, s, files, zipFile)
	}
}
