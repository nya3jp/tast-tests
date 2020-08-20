// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"os"
	"path/filepath"
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
		Data:         []string{"Texts.zip"},
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

	// ZIP files names.
	const zipFile = "Texts.zip"

	// Open the Downloads folder.
	if err := files.OpenDownloads(ctx); err != nil {
		s.Fatal("Cannot open Downloads folder: ", err)
	}

	// Load ZIP file.
	zipFileLocation := filepath.Join(filesapp.DownloadPath, zipFile)

	if err := fsutil.CopyFile(s.DataPath(zipFile), zipFileLocation); err != nil {
		s.Fatalf("Cannot copy ZIP file to %s: %s", zipFileLocation, err)
	}
	defer os.Remove(zipFileLocation)

	testMountingSingleZipFile(ctx, s, files, zipFile)
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

	// Checks that the tree item corresponding to the previously mounted ZIP file was removed.
	params = ui.FindParams{
		Name: zipFile,
		Role: ui.RoleTypeTreeItem,
	}

	if err = files.Root.WaitUntilDescendantGone(ctx, params, 5*time.Second); err != nil {
		s.Fatalf("%s is still listed in Files app: %v", zipFile, err)
	}
}

func testMountingSingleZipFile(ctx context.Context, s *testing.State, files *filesapp.FilesApp, zipFile string) {
	// Select ZIP file.
	if err := files.WaitForFile(ctx, zipFile, 5*time.Second); err != nil {
		s.Fatal("Waiting for test file failed: ", err)
	}

	// Click ZIP file and wait for Open button in top bar.
	if err := files.SelectFile(ctx, zipFile); err != nil {
		s.Fatal("Cannot mount ZIP file: ", err)
	}

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
		s.Fatal("Cannot open mounted ZIP file: ", err)
	}

	unmountZipFile(ctx, s, files, zipFile)
}
