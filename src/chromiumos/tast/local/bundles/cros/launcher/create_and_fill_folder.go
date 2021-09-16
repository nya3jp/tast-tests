// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"io/ioutil"
	"os"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CreateAndFillFolder,
		Desc: "Test adding items to a folder in the launcher",
		Contacts: []string{
			"cros-system-ui-eng@google.com",
			"chromeos-sw-engprod@google.com",
			"mmourgos@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      4 * time.Minute,
	})
}

// CreateAndFillFolder tests that a folder can be filled to the maximum allowed size.
func CreateAndFillFolder(ctx context.Context, s *testing.State) {
	extDirBase, err := ioutil.TempDir("", "")
	if err != nil {
		s.Fatal("Failed to create a tempdir: ", err)
	}

	// Create 50 fake apps to be used for filling a folder.
	numFakeApps := 50
	dirs, err := ash.PrepareFakeApps(extDirBase, numFakeApps, nil)
	if err != nil {
		s.Fatal("Failed to prepare fake apps: ", err)
	}
	defer os.RemoveAll(extDirBase)

	opts := make([]chrome.Option, 0, numFakeApps)
	for _, dir := range dirs {
		opts = append(opts, chrome.UnpackedExtension(dir))
	}

	// Creating fake apps and logging into a new session in this test ensures that enough apps will be available to folder.
	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure in clamshell mode: ", err)
	}
	defer cleanup(ctx)

	// Open the Launcher and go to the apps grid page.
	if err := launcher.OpenExpandedView(tconn)(ctx); err != nil {
		s.Fatal("Failed to open Expanded Application list view: ", err)
	}

	if err := launcher.CreateFolder(ctx, tconn); err != nil {
		s.Fatal("Failed to create folder app: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	// Rename the newly created folder so that it is only used for this test and
	// gets ignored for any following folder launcher tests.
	if err := launcher.RenameFolder(tconn, kb, "Unnamed", "CreateAndFillFolder")(ctx); err != nil {
		s.Fatal("Failed to rename folder to NewName: ", err)
	}
	filledFolder := nodewith.Name("Folder CreateAndFillFolder").ClassName(launcher.ExpandedItemsClass)

	// The folder already has 2 items. Add 46 more items to get to the maximum folder size of 48 apps.
	if err := launcher.AddItemsToFolder(ctx, tconn, filledFolder, 46); err != nil {
		s.Fatal("Failed to add items to folder: ", err)
	}

	// Check that the number of apps in the folder is 48.
	size, err := launcher.GetFolderSize(ctx, tconn, filledFolder)
	if err != nil {
		s.Fatal("Failed to get the folder size: ", err)
	}
	if size != 48 {
		s.Fatalf("Unexpected number of items in folder, got %d, want %d", size, 48)
	}

	// Attempt to add one more item to the folder.
	if err := launcher.AddItemsToFolder(ctx, tconn, filledFolder, 1); err != nil {
		s.Fatal("Failed to add items to folder: ", err)
	}

	// Because the folder was already filled to the maximum size, the number of apps in the folder should still be 48.
	// Check that the folder size remains at the max of 48.
	size, err = launcher.GetFolderSize(ctx, tconn, filledFolder)
	if err != nil {
		s.Fatal("Failed to get the folder size: ", err)
	}
	if size != 48 {
		s.Fatalf("Unexpected number of items in folder, got %d, want %d", size, 48)
	}
}
