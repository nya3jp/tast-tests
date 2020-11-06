// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/screenshot"
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

	// Open the Files App.
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Launching the Files App failed: ", err)
	}
	defer files.Release(ctx)

	// If test fails dump UI tree and take a screenshot.
	// Always attempt to remove any leftover files even if successful.
	var createdFileOrFolderPaths []string
	defer func() {
		if s.HasError() {
			faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
			if err := screenshot.CaptureChrome(ctx, cr, filepath.Join(s.OutDir(), "screenshot.png")); err != nil {
				s.Log("Failed to take screenshot: ", err)
			}
		}

		for _, path := range createdFileOrFolderPaths {
			if err := os.RemoveAll(path); err != nil {
				s.Log("Failed to remove path: ", path)
			}
		}
	}()

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
		if err := files.CreateFolder(ctx, testFolderName); err != nil {
			s.Fatalf("Failed creating a folder %q inside %q: %v", testFolderName, folder.path, err)
		}
		testFolderLocation := filepath.Join(folder.path, testFolderName)
		createdFileOrFolderPaths = append(createdFileOrFolderPaths, testFolderLocation)

		// Create a test file inside the subfolder.
		testFileLocation := filepath.Join(folder.path, testFolderName, testFileName)
		if err := ioutil.WriteFile(testFileLocation, []byte("blahblah"), 0644); err != nil {
			s.Fatalf("Failed creating file %q: %v", testFileLocation, err)
		}
		createdFileOrFolderPaths = append(createdFileOrFolderPaths, testFileLocation)

		// Wait for folder to be visible in list view.
		params := ui.FindParams{
			Name: testFolderName,
			Role: ui.RoleTypeStaticText,
		}
		if err := files.Root.WaitUntilDescendantExists(ctx, params, 15*time.Second); err != nil {
			s.Fatalf("Failed waiting for %q folder to be clickable: %v", testFolderName, err)
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
