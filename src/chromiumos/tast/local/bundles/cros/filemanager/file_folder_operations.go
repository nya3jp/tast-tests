// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"io/ioutil"
	"path/filepath"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FileFolderOperations,
		Desc: "Basic file folder operations work in My Files and Downloads",
		Contacts: []string{
			"benreich@chromium.org",
			"chromeos-files-syd@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

const (
	testFolderName = "test_folder"
	testFileName   = "test_file.txt"
)

func FileFolderOperations(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	// Open the test API.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Open the Files App.
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Launching the Files App failed: ", err)
	}
	// Instead of closing the Files App, just release the memory reference.
	// Otherwise, when this test fails, the screenshot will be of an empty desktop/closing app.
	defer files.Root.Release(ctx)

	// Get a handle to the input keyboard
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard handle: ", err)
	}
	defer kb.Close()

	for _, folder := range []struct {
		item string
		path string
	}{
		{filesapp.MyFiles, filesapp.MyFilesPath},
		{filesapp.Downloads, filesapp.DownloadPath},
	} {
		// Click on folder in the directory tree.
		if err := files.LeftClickItem(ctx, folder.item, ui.RoleTypeTreeItem); err != nil {
			s.Fatalf("Failed opening directory %q: %v", folder.item, err)
		}

		// Create a subfolder.
		if err := files.CreateFolder(ctx, testFolderName, kb); err != nil {
			s.Fatalf("Failed creating a folder %q inside %q: %v", testFolderName, folder.path, err)
		}

		// Create a test file inside the subfolder.
		testFileLocation := filepath.Join(folder.path, testFolderName, testFileName)
		if err := ioutil.WriteFile(testFileLocation, []byte("blahblah"), 0644); err != nil {
			s.Fatalf("Failed creating file %q: %v", testFileLocation, err)
		}

		// Open the subfolder.
		if err := files.OpenFile(ctx, testFolderName); err != nil {
			s.Fatalf("Failed clicking %q: %v", testFolderName, err)
		}

		// Delete the file within the subfolder.
		if err := files.DeleteFileOrFolder(ctx, testFileName); err != nil {
			s.Fatalf("Failed deleting file %q: %v", testFileName, err)
		}

		// Open the enclosing folder that was navigated to.
		if err := files.SelectEnclosingFolder(ctx); err != nil {
			s.Fatalf("Failed clicking enclosing folder %q via breadcrumbs: %v", testFolderName, err)
		}

		// Delete the subfolder that was previously created.
		if err := files.DeleteFileOrFolder(ctx, testFolderName); err != nil {
			s.Fatalf("Failed deleteing folder %q: %v", testFolderName, err)
		}
	}
}
