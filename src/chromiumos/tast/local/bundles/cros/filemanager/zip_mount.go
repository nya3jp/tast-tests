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

	"github.com/shirou/gopsutil/process"
	"golang.org/x/sys/unix"

	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// TestFunc contains the contents of the test itself.
type testFunc func(ctx context.Context, s *testing.State, files *filesapp.FilesApp, zipFiles []string)

// TestEntry contains the function used in the test along with the names of the ZIP files the test is using.
type testEntry struct {
	TestCase testFunc
	ZipFiles []string
	IsGuest  bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ZipMount,
		LacrosStatus: testing.LacrosVariantUnknown,
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
				IsGuest:  false,
			},
		}, {
			Name: "mount_single_rar",
			Val: testEntry{
				TestCase: testMountingSingleZipFile,
				ZipFiles: []string{"Texts.rar"},
				IsGuest:  false,
			},
		}, {
			Name: "mount_single_zip",
			Val: testEntry{
				TestCase: testMountingSingleZipFile,
				ZipFiles: []string{"Texts.zip"},
				IsGuest:  false,
			},
		}, {
			Name: "cancel_multiple",
			Val: testEntry{
				TestCase: testCancelingMultiplePasswordDialogs,
				ZipFiles: []string{"Encrypted_AES-256.zip", "Encrypted_ZipCrypto.zip"},
				IsGuest:  false,
			},
		}, {
			Name: "mount_multiple",
			Val: testEntry{
				TestCase: testMountingMultipleZipFiles,
				ZipFiles: []string{"Encrypted_AES-256.zip", "Encrypted_ZipCrypto.zip", "Texts.zip"},
				IsGuest:  false,
			},
		}, {
			Name: "mount_single_7z_guest",
			Val: testEntry{
				TestCase: testMountingSingleZipFile,
				ZipFiles: []string{"Texts.7z"},
				IsGuest:  true,
			},
		}, {
			Name: "mount_single_rar_guest",
			Val: testEntry{
				TestCase: testMountingSingleZipFile,
				ZipFiles: []string{"Texts.rar"},
				IsGuest:  true,
			},
		}, {
			Name: "mount_single_zip_guest",
			Val: testEntry{
				TestCase: testMountingSingleZipFile,
				ZipFiles: []string{"Texts.zip"},
				IsGuest:  true,
			},
		}, {
			Name: "cancel_multiple_guest",
			Val: testEntry{
				TestCase: testCancelingMultiplePasswordDialogs,
				ZipFiles: []string{"Encrypted_AES-256.zip", "Encrypted_ZipCrypto.zip"},
				IsGuest:  true,
			},
		}, {
			Name: "mount_multiple_guest",
			Val: testEntry{
				TestCase: testMountingMultipleZipFiles,
				ZipFiles: []string{"Encrypted_AES-256.zip", "Encrypted_ZipCrypto.zip", "Texts.zip"},
				IsGuest:  true,
			},
		}},
	})
}

// getCryptohomeNamespaceMounterPID returns the PID of the 'cryptohome-namespace-mounter' process,
// if found.
func getCryptohomeNamespaceMounterPID() (int, error) {
	const exePath = "/usr/sbin/cryptohome-namespace-mounter"

	all, err := process.Pids()
	if err != nil {
		return -1, err
	}

	for _, pid := range all {
		if proc, err := process.NewProcess(pid); err != nil {
			// Assume that the process exited.
			continue
		} else if exe, err := proc.Exe(); err == nil && exe == exePath {
			return int(pid), nil
		}
	}
	return -1, errors.New("mounter process not found")
}

func ZipMount(ctx context.Context, s *testing.State) {
	testParams := s.Param().(testEntry)
	zipFiles := testParams.ZipFiles
	isGuest := testParams.IsGuest

	var cr *chrome.Chrome
	var err error

	if isGuest {
		// TODO(nigeltao): remove "FilesArchivemount" after it gets flipped to
		// enabled-by-default (scheduled for M94) and before the feature flag
		// expires (scheduled for M100). crbug.com/1216245
		cr, err = chrome.New(ctx, chrome.GuestLogin(), chrome.EnableFeatures("FilesArchivemount"))
		if err != nil {
			s.Fatal("Login failed: ", err)
		}
		// chrome.Chrome.Close() will not log the user out.
		defer upstart.RestartJob(ctx, "ui")

		nsPath := "/proc/1/ns/mnt"
		if mounterPid, err := getCryptohomeNamespaceMounterPID(); err == nil {
			nsPath = fmt.Sprintf("/proc/%d/ns/mnt", mounterPid)
		}

		// Guest sessions can be mounted in a non-root mount namespace
		// so the test needs to perform checks in that same namespace.
		s.Log("Attempting to open Chrome mount namespace at ", nsPath)
		chromeNsFd, err := unix.Open(nsPath, unix.O_CLOEXEC, unix.O_RDWR)
		if err != nil {
			s.Fatal("Opening Chrome mount namespace failed: ", err)
		}
		defer unix.Close(chromeNsFd)

		// Open root mount namespace to be able to switch back to it.
		rootNsFd, err := unix.Open("/proc/1/ns/mnt", unix.O_CLOEXEC, unix.O_RDONLY)
		if err != nil {
			s.Fatal("Opening root mount namespace failed: ", err)
		}
		defer unix.Close(rootNsFd)

		// Ensure we can successfully call setns(2) by first calling unshare(2)
		// which will make this thread's view of mounts distinct from the root's.
		if err := unix.Unshare(unix.CLONE_NEWNS); err != nil {
			s.Fatal("Unsharing mount namespace failed: ", err)
		}
		// As soon as we've successfully called unshare(2) ensure we switch back
		// to the original namespace.
		defer unix.Setns(rootNsFd, unix.CLONE_NEWNS)

		if err := unix.Setns(chromeNsFd, unix.CLONE_NEWNS); err != nil {
			s.Fatalf("Entering Chrome mount namespace at %s failed: %v", nsPath, err)
		}
	} else {
		// TODO(nigeltao): remove "FilesArchivemount" after it gets flipped to
		// enabled-by-default (scheduled for M94) and before the feature flag
		// expires (scheduled for M100). crbug.com/1216245
		cr, err = chrome.New(ctx, chrome.EnableFeatures("FilesArchivemount"))
		if err != nil {
			s.Fatal("Cannot start Chrome: ", err)
		}
		defer cr.Close(ctx)
	}

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

	// Find and click the 'Name' button to order the file entries alphabetically.
	orderByNameButton := nodewith.Name("Name").Role(role.Button)
	if err := files.LeftClick(orderByNameButton)(ctx); err != nil {
		s.Fatal("Cannot find and click 'Name' button: ", err)
	}

	// Wait until the ZIP files are correctly ordered in the list box.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		listBox := nodewith.Role(role.ListBox).Focusable().Multiselectable().Vertical()
		listBoxOption := nodewith.Role(role.ListBoxOption).Ancestor(listBox)
		nodes, err := files.NodesInfo(ctx, listBoxOption)
		if err != nil {
			return testing.PollBreak(err)
		}

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

	testParams.TestCase(ctx, s, files, zipFiles)
}

// waitUntilPasswordDialogExists waits for the password dialog to display for a specific encrypted ZIP file.
func waitUntilPasswordDialogExists(ctx context.Context, files *filesapp.FilesApp, fileName string) error {
	// Look for expected file name label within the current dialog.
	passwordDialog := nodewith.Role(role.Dialog)
	node := nodewith.Name(fileName).Role(role.StaticText).Ancestor(passwordDialog)
	return files.WithTimeout(15 * time.Second).WaitUntilExists(node)(ctx)

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

	// Wait until the password dialog is active for the first encrypted ZIP archive.
	if err := waitUntilPasswordDialogExists(ctx, files, zipFiles[0]); err != nil {
		s.Fatalf("Cannot find password dialog for %s : %v", zipFiles[0], err)
	}

	// Press Esc.
	if err := ew.Accel(ctx, "Esc"); err != nil {
		s.Fatal("Cannot press 'Esc': ", err)
	}

	// Wait until the password dialog is active for the second encrypted ZIP archive.
	if err := waitUntilPasswordDialogExists(ctx, files, zipFiles[1]); err != nil {
		s.Fatalf("Cannot find password dialog for %s : %v", zipFiles[1], err)
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

	// Select the 3 encrypted ZIP files.
	if err := files.SelectMultipleFiles(ew, zipFiles...)(ctx); err != nil {
		s.Fatal("Cannot perform multi-selection on the encrypted files: ", err)
	}

	// Press Enter.
	if err := ew.Accel(ctx, "Enter"); err != nil {
		s.Fatal("Cannot press 'Enter': ", err)
	}

	// Wait until the password dialog is active for the first encrypted ZIP archive.
	if err := waitUntilPasswordDialogExists(ctx, files, zipFiles[0]); err != nil {
		s.Fatalf("Cannot find password dialog for %s : %v", zipFiles[0], err)
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

	// Check that the password dialog is still active for the first encrypted ZIP archive.
	if err := waitUntilPasswordDialogExists(ctx, files, zipFiles[0]); err != nil {
		s.Fatalf("Cannot find password dialog for %s : %v", zipFiles[0], err)
	}

	// Check that "Invalid password" displays on the UI.
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

	// Check that the password dialog is active for the second encrypted ZIP archive.
	if err := waitUntilPasswordDialogExists(ctx, files, zipFiles[1]); err != nil {
		s.Fatalf("Cannot find password dialog for %s : %v", zipFiles[1], err)
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
