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
		Func:         CreateAndFillFolder,
		Desc:         "Test adding items to a folder in the launcher",
		Contacts:     []string{"cros-launcher-prod-notifications@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedInWith100FakeApps",
	})
}

// CreateAndFillFolder tests that a folder can be filled to the maximum allowed size.
func CreateAndFillFolder(ctx context.Context, s *testing.State) {
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
	ac := uiauto.New(tconn)
	if err := ac.Retry(2, launcher.OpenExpandedView(tconn))(ctx); err != nil {
		s.Fatal("Failed to open Expanded Application list view: ", err)
	}

	if err := ac.Retry(3, launcher.CreateFolder(ctx, tconn, ac))(ctx); err != nil {
		s.Fatal("Failed to create colder app: ", err)
	}

	// Add all 18 app items on this app list page to the folder.
	for i := 0; i < 18; i++ {
		launcher.DragIconToIcon(ctx, tconn, ac)(ctx)
	}

	// Move the folder to the next page and fill with apps completely.
	launcher.DragIconToNextPage(tconn)(ctx)
	for i := 0; i < 19; i++ {
		launcher.DragIconToIcon(ctx, tconn, ac)(ctx)
	}

	// Move the folder to the next page and fill the folder apps completely.
	launcher.DragIconToNextPage(tconn)(ctx)
	for i := 0; i < 9; i++ {
		launcher.DragIconToIcon(ctx, tconn, ac)(ctx)
	}

	// Check that the number of apps in the folder is 48.
	var size int
	size, err = launcher.GetFolderSize(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the folder size: ", err)
	}
	if size != 48 {
		s.Fatalf("Unexpected number of items in folder, got %d, want %d", size, 48)
	}

	// Test adding one more item to the folder, and then check that the folder
	// still has only 48 items.
	launcher.DragIconToIcon(ctx, tconn, ac)(ctx)

	// Check that the number of apps in the folder is 48.
	size, err = launcher.GetFolderSize(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the folder size: ", err)
	}
	if size != 48 {
		s.Fatalf("Unexpected number of items in folder, got %d, want %d", size, 48)
	}
}
