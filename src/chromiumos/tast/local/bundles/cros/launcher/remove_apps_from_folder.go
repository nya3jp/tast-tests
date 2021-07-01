// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: RemoveAppsFromFolder,
		Desc: "Test removing items from a folder in the launcher",
		Contacts: []string{
			"cros-system-ui-eng@google.com",
			"chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedInWith100FakeApps",
	})
}

// RemoveAppsFromFolder tests that items can be removed from a folder.
func RemoveAppsFromFolder(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure in clamshell mode: ", err)
	}
	defer cleanup(ctx)

	// Open the Launcher and go to Apps list page.
	ui := uiauto.New(tconn)
	if err := launcher.OpenExpandedView(tconn)(ctx); err != nil {
		s.Fatal("Failed to open Expanded Application list view: ", err)
	}

	if err := launcher.CreateFolder(ctx, tconn)(ctx); err != nil {
		s.Fatal("Failed to create folder app: ", err)
	}

	// Add 5 app items to the folder.
	if err := launcher.AddItemsToFolder(ctx, tconn, launcher.UnnamedFolderFinder, 5); err != nil {
		s.Fatal("Failed to add items to folder: ", err)
	}

	// Check that the folder has 7 items.
	size, err := launcher.GetFolderSize(ctx, tconn, launcher.UnnamedFolderFinder)
	if err != nil {
		s.Fatal("Failed to get the folder size: ", err)
	}
	if size != 7 {
		s.Fatalf("Unexpected number of items in folder, got %d, want %d", size, 7)
	}

	// Remove 3 items from the folder.
	for i := 0; i < 3; i++ {
		if err := launcher.RemoveIconFromFolder(tconn)(ctx); err != nil {
			s.Fatal("Failed to remove icon from folder: ", err)
		}
	}

	// Check that the folder has 4 items.
	size, err = launcher.GetFolderSize(ctx, tconn, launcher.UnnamedFolderFinder)
	if err != nil {
		s.Fatal("Failed to get the folder size: ", err)
	}
	if size != 4 {
		s.Fatalf("Unexpected number of items in folder, got %d, want %d", size, 4)
	}

	// Remove 3 items from the folder.
	for i := 0; i < 3; i++ {
		if err := launcher.RemoveIconFromFolder(tconn)(ctx); err != nil {
			s.Fatal("Failed to remove icon from folder: ", err)
		}
	}

	// Check that there is no longer a folder.
	if err := ui.Gone(launcher.UnnamedFolderFinder)(ctx); err != nil {
		s.Fatal("Folder exists when it should not: ", err)
	}
}
