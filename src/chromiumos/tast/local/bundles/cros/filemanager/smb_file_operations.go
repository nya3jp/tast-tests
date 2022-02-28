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
		}, {
			Name: "delete",
			Val:  testDeleteOperation,
		}, {
			Name: "unmount",
			Val:  testUnmountOperation,
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

	// Get a handle to the input keyboard.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard handle: ", err)
	}
	defer kb.Close()

	// A
	ui := uiauto.New(tconn)
	if err := uiauto.Combine("add the SMB file share via Files context menu",
		files.ClickMoreMenuItem("Services", "SMB file share"),
		smb.AddFileShareAction(ui, kb, true /*=rememberPassword*/, smb.GuestShareName, "" /*=username*/, "" /*=password*/),
	)(ctx); err != nil {
		s.Fatal("Failed to click add SMB share: ", err)
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
		files.OpenPath(filesapp.FilesTitlePrefix+smb.GuestShareName, smb.GuestShareName),
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
		files.OpenDir(smb.GuestShareName, filesapp.FilesTitlePrefix+smb.GuestShareName),
		files.RenameFile(kb, textFile, expectedFile),
		files.WaitForFile(expectedFile),
	)(ctx); err != nil {
		s.Fatal("Failed to rename text file: ", err)
	}
}

func testDeleteOperation(ctx context.Context, kb *input.KeyboardEventWriter, s *testing.State, fixture smb.FixtureData, files *filesapp.FilesApp) {
	const textFile = "test_delete_file.txt"
	testFileLocation := filepath.Join(fixture.GuestSharePath, textFile)
	if err := createTestFile(testFileLocation); err != nil {
		s.Fatalf("Failed to create file %q: %s", testFileLocation, err)
	}
	defer os.Remove(testFileLocation)

	if err := uiauto.Combine("delete existing file on Samba share",
		files.OpenDir(smb.GuestShareName, filesapp.FilesTitlePrefix+smb.GuestShareName),
		files.DeleteFileOrFolder(kb, textFile),
	)(ctx); err != nil {
		s.Fatal("Failed to delete text file: ", err)
	}
}

func testUnmountOperation(ctx context.Context, kb *input.KeyboardEventWriter, s *testing.State, fixture smb.FixtureData, files *filesapp.FilesApp) {
	const textFile = "test.txt"
	testFileLocation := filepath.Join(fixture.GuestSharePath, textFile)
	if err := createTestFile(testFileLocation); err != nil {
		s.Fatalf("Failed to create file %q: %s", testFileLocation, err)
	}
	defer os.Remove(testFileLocation)

	guestshareFinder := nodewith.Name(smb.GuestShareName).Role(role.StaticText)
	if err := uiauto.Combine("unmount a mounted Samba share",
		files.LeftClickUntil(guestshareFinder, files.WaitForFile(textFile)),
		kb.AccelAction("Ctrl+Shift+E"),
		files.WaitUntilGone(guestshareFinder),
	)(ctx); err != nil {
		s.Fatal("Failed to unmount the Samba share: ", err)
	}
}

func createTestFile(path string) error {
	return ioutil.WriteFile(path, []byte("blahblah"), 0644)
}
