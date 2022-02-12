// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/smb"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SMBFileOperations,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify Files app can mount an SMB share and verify the contents",
		Contacts: []string{
			"benreich@chromium.org",
			"chromeos-files-syd@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "smbStarted",
		Params: []testing.Param{{
			Name: "copy",
			Val:  testCopyOperation,
		}, {
			Name: "rename",
			Val:  testRenameOperation,
		}},
	})
}

type smbFileOperationTestFunc = func(ctx context.Context, kb *input.KeyboardEventWriter, s *testing.State, fixture smb.FixtureData, files *filesapp.FilesApp)

func SMBFileOperations(ctx context.Context, s *testing.State) {
	fixt := s.FixtValue().(smb.FixtureData)
	testFunc := s.Param().(smbFileOperationTestFunc)

	// Open the test API.
	tconn, err := fixt.Chrome.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create the Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Launch the files application.
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch the Files app: ", err)
	}

	// Open the Add SMB share dialog and focus the first text field.
	ui := uiauto.New(tconn)
	fileShareURLTextBox := nodewith.Name("File share URL").Role(role.TextField)
	if err := uiauto.Combine("Click add SMB file share",
		files.ClickMoreMenuItem("Services", "SMB file share"),
		ui.WaitForLocation(fileShareURLTextBox),
		ui.LeftClick(fileShareURLTextBox))(ctx); err != nil {
		s.Fatal("Failed to click add SMB share: ", err)
	}

	// Get a handle to the input keyboard.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard handle: ", err)
	}
	defer kb.Close()

	if err := kb.Type(ctx, `\\localhost\guestshare`); err != nil {
		s.Fatal("Failed to enter the new SMB file share path: ", err)
	}

	if err := kb.Accel(ctx, "Enter"); err != nil {
		s.Fatal("Failed to press enter: ", err)
	}

	testFunc(ctx, kb, s, fixt, files)
}

func testCopyOperation(ctx context.Context, kb *input.KeyboardEventWriter, s *testing.State, fixture smb.FixtureData, files *filesapp.FilesApp) {
	const textFile = "test.txt"
	testFileLocation := filepath.Join(filesapp.DownloadPath, textFile)
	if err := createTestFile(testFileLocation); err != nil {
		s.Fatalf("Failed to create file %q: %s", testFileLocation, err)
	}
	defer os.Remove(testFileLocation)

	if err := uiauto.Combine("wait for SMB to mount",
		files.OpenDownloads(),
		files.ClickContextMenuItem(textFile, filesapp.Copy),
		files.OpenPath("Files - guestshare", "guestshare"),
		kb.AccelAction("Ctrl+V"),
		files.WaitForFile(textFile),
	)(ctx); err != nil {
		s.Fatal("Failed to wait for SMB to mount: ", err)
	}
}

func testRenameOperation(ctx context.Context, kb *input.KeyboardEventWriter, s *testing.State, fixture smb.FixtureData, files *filesapp.FilesApp) {
	const (
		textFile     = "test_old_rename.txt"
		expectedFile = "text_new_rename.txt"
	)
	testFileLocation := filepath.Join(fixture.GuestSharePath, textFile)
	if err := createTestFile(testFileLocation); err != nil {
		s.Fatalf("Failed to create file %q: %s", testFileLocation, err)
	}
	defer os.Remove(testFileLocation)

	if err := uiauto.Combine("rename existing file on Samba share",
		files.OpenDir("guestshare", filesapp.FilesTitlePrefix+"guestshare"),
		files.RenameFile(kb, textFile, expectedFile),
		files.WaitForFile(expectedFile),
	)(ctx); err != nil {
		s.Fatal("Failed to rename text file: ", err)
	}
}

func createTestFile(path string) error {
	return ioutil.WriteFile(path, []byte("blahblah"), 0644)
}
