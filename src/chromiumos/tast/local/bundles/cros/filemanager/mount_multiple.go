// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// Set of zipFileNames for which we still expect a password dialog: the boolean value has no meaning.
var expectedDialogFileNames map[string]bool

// Keeps track of what filename was shown on the last password dialog.
var currentDialogFileName string

// waitUntilPasswordDialogExistsAction defines the action for waitUntilPasswordDialogExists.
func waitUntilPasswordDialogExistsAction(files *filesapp.FilesApp, expectedDialogFileNames map[string]bool, expectInvalidPassword bool) action.Action {
	return func(ctx context.Context) error {
		dialogFileName, err := waitUntilPasswordDialogExists(ctx, files, expectedDialogFileNames, expectInvalidPassword)
		currentDialogFileName = dialogFileName
		return err
	}
}

// removeCurrentFileNameFromExpectedDialogs defines the action to remove the current password dialog file name from the set of expected dialogs.
func removeCurrentFileNameFromExpectedDialogs(s *testing.State) action.Action {
	return func(ctx context.Context) error {
		if _, exists := expectedDialogFileNames[currentDialogFileName]; !exists {
			s.Fatal("Unexpected file name: ")
		}
		delete(expectedDialogFileNames, currentDialogFileName)
		return nil
	}
}

// TestFunc contains the contents of the test itself.
type testFunc func(ctx context.Context, s *testing.State, files *filesapp.FilesApp, zipFiles []string)

// TestEntry contains the function used in the test along with the names of the ZIP files the test is using.
type testEntry struct {
	TestCase testFunc
	ZipFiles []string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         MountMultiple,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that Files App can mount multiple archives in one go",
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
			"Texts.zip",
		},
		Timeout: 2 * time.Minute,
		Params: []testing.Param{{
			Name: "cancel_multiple",
			Val: testEntry{
				TestCase: cancelMultiplePasswordDialogs,
				ZipFiles: []string{"Encrypted_AES-256.zip", "Encrypted_ZipCrypto.zip"},
			},
		}, {
			Name: "mount_multiple",
			Val: testEntry{
				TestCase: mountMultipleZipFiles,
				ZipFiles: []string{"Encrypted_AES-256.zip", "Encrypted_ZipCrypto.zip", "Texts.zip"},
			},
		}},
	})
}

func MountMultiple(ctx context.Context, s *testing.State) {
	testParams := s.Param().(testEntry)
	zipFiles := testParams.ZipFiles

	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Cannot start Chrome: ", err)
	}
	defer cr.Close(ctx)

	downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to retrieve users Downloads path: ", err)
	}

	// Load ZIP files.
	for _, zipFile := range zipFiles {
		zipFileLocation := filepath.Join(downloadsPath, zipFile)
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
func waitUntilPasswordDialogExists(ctx context.Context, files *filesapp.FilesApp, processedZipFiles map[string]bool, expectInvalidPassword bool) (string, error) {
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

		// Check "Invalid password" if that's expected.
		if expectInvalidPassword {
			if err := files.WithTimeout(15 * time.Second).WaitUntilExists(nodewith.Name("Invalid password").Role(role.StaticText))(ctx); err != nil {
				return errors.Wrap(err, "cannot invalid password message")
			}
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
	// The node for the mounted ZIP file in the directory tree.
	zipFileNode := nodewith.Name(zipFile).Role(role.TreeItem)

	// The test ZIP files are all expected to contain a single "Texts" folder.
	var zipContentDirectoryLabel = "Texts"

	// The eject button within the appropriate tree item.
	ejectButton := nodewith.Name("Eject device").Role(role.Button).Ancestor(zipFileNode)

	// Ensure that the ZIP file has been mounted correctly,
	if err := uiauto.Combine("check and unmount ZIP file",
		files.WithTimeout(5*time.Second).WaitUntilExists(zipFileNode),
		files.LeftClick(zipFileNode),
		files.WithTimeout(5*time.Second).WaitForFile(zipContentDirectoryLabel),
		files.WithTimeout(5*time.Second).WaitUntilExists(ejectButton),
		files.LeftClick(ejectButton),
		files.WithTimeout(5*time.Second).WaitUntilFileGone(zipContentDirectoryLabel),
	)(ctx); err != nil {
		s.Fatalf("Cannot check and unmount ZIP file %q: %v", zipFile, err)
	}
}

func cancelMultiplePasswordDialogs(ctx context.Context, s *testing.State, files *filesapp.FilesApp, zipFiles []string) {
	// Define keyboard to perform keyboard shortcuts.
	ew, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Cannot create keyboard: ", err)
	}
	defer ew.Close()

	expectedDialogFileNames = make(map[string]bool)
	for _, zipFile := range zipFiles {
		expectedDialogFileNames[zipFile] = true
	}

	currentDialogFileName = ""

	if err := uiauto.Combine("mount multiple zip files",
		// Select the 2 encrypted ZIP files.
		files.SelectMultipleFiles(ew, zipFiles...),
		// Press Enter.
		ew.AccelAction("Enter"),
		// Wait until the password dialog is active for one of the encrypted ZIP archives.
		waitUntilPasswordDialogExistsAction(files, expectedDialogFileNames, false),
		// We don't expect another password dialog for this particular file.
		removeCurrentFileNameFromExpectedDialogs(s),
		// Press Esc.
		ew.AccelAction("Esc"),
		// Wait until the password dialog is active for the second encrypted ZIP archive.
		waitUntilPasswordDialogExistsAction(files, expectedDialogFileNames, false),
		// Click the 'Cancel' button.
		files.WithTimeout(5*time.Second).WaitUntilExists(nodewith.Name("Cancel").Role(role.Button)),
		files.LeftClick(nodewith.Name("Cancel").Role(role.Button)),
		// Checks that the password dialog is not displayed anymore.
		files.WithTimeout(5*time.Second).WaitUntilGone(nodewith.Role(role.Dialog)),
	)(ctx); err != nil {
		s.Fatal("Cannot mount multiple ZIP files: ", err)
	}
}

func mountMultipleZipFiles(ctx context.Context, s *testing.State, files *filesapp.FilesApp, zipFiles []string) {
	// Define keyboard to perform keyboard shortcuts.
	ew, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Cannot create keyboard: ", err)
	}
	defer ew.Close()

	expectedDialogFileNames = make(map[string]bool)
	for _, zipFile := range zipFiles {
		expectedDialogFileNames[zipFile] = true
	}

	currentDialogFileName = ""

	if err := uiauto.Combine("mount multiple zip files",
		// Select the 3 ZIP files. 2 of them are encrypted.
		files.SelectMultipleFiles(ew, zipFiles...),
		// Press Enter.
		ew.AccelAction("Enter"),
		// Wait until the password dialog is active for one of the encrypted ZIP archives.
		waitUntilPasswordDialogExistsAction(files, expectedDialogFileNames, false),
		// Enter wrong password.
		files.WithTimeout(5*time.Second).WaitUntilExists(nodewith.Role(role.TextField).Editable().Focusable().Protected().Focused()),
		// Type password.
		ew.TypeAction("wrongPassword"),
		// Validate password.
		ew.AccelAction("Enter"),
		// Check that the password dialog is still active.
		waitUntilPasswordDialogExistsAction(files, expectedDialogFileNames, true),
		// Enter right password.
		ew.TypeAction("password"),
		// Validate password.
		ew.AccelAction("Enter"),
		// We don't expect another password dialog for this particular file.
		removeCurrentFileNameFromExpectedDialogs(s),
		// Check that the password dialog is active for the second encrypted ZIP archive.
		waitUntilPasswordDialogExistsAction(files, expectedDialogFileNames, false),
		// Enter right password.
		ew.TypeAction("password"),
		// Click Unlock.
		files.WithTimeout(5*time.Second).WaitUntilExists(nodewith.Name("Unlock").Role(role.Button)),
		files.LeftClick(nodewith.Name("Unlock").Role(role.Button)),
		files.WithTimeout(5*time.Second).WaitUntilGone(nodewith.Role(role.Dialog)),
	)(ctx); err != nil {
		s.Fatal("Cannot mount multiple ZIP files: ", err)
	}

	// Check that the 3 zip files have been mounted correctly and unmount them.
	for _, zipFile := range zipFiles {
		checkAndUnmountZipFile(ctx, s, files, zipFile)
	}
}
