// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"os"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RemoveAppsFromFolder,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Test removing items from a folder in the launcher",
		Contacts: []string{
			"cros-system-ui-eng@google.com",
			"chromeos-sw-engprod@google.com",
			"mmourgos@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "productivity_launcher_clamshell_mode",
			Val:  launcher.TestCase{ProductivityLauncher: true, TabletMode: false},
		}, {
			Name: "clamshell_mode",
			Val:  launcher.TestCase{ProductivityLauncher: false, TabletMode: false},
		}, {
			Name:              "productivity_launcher_tablet_mode",
			Val:               launcher.TestCase{ProductivityLauncher: true, TabletMode: true},
			ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		}, {
			Name:              "tablet_mode",
			Val:               launcher.TestCase{ProductivityLauncher: false, TabletMode: true},
			ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		}},
	})
}

// RemoveAppsFromFolder tests that items can be removed from a folder.
func RemoveAppsFromFolder(ctx context.Context, s *testing.State) {
	// Create 10 fake apps and get the the options to add to the new chrome session.
	opts, extDirBase, err := ash.GeneratePrepareFakeAppsOptions(10)
	if err != nil {
		s.Fatal("Failed to create 10 fake apps")
	}
	defer os.RemoveAll(extDirBase)

	testCase := s.Param().(launcher.TestCase)
	productivityLauncher := testCase.ProductivityLauncher
	if productivityLauncher {
		opts = append(opts, chrome.EnableFeatures("ProductivityLauncher"))
	} else {
		opts = append(opts, chrome.DisableFeatures("ProductivityLauncher"))
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

	tabletMode := testCase.TabletMode
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, tabletMode)
	if err != nil {
		s.Fatalf("Failed to ensure tablet mode state %t: %v", tabletMode, err)
	}
	defer cleanup(ctx)

	if !tabletMode {
		if err := ash.WaitForLauncherState(ctx, tconn, ash.Closed); err != nil {
			s.Fatal("Launcher not closed after transition to clamshell mode: ", err)
		}
	}

	usingBubbleLauncher := productivityLauncher && !tabletMode
	// Open the Launcher and go to Apps list page.
	if usingBubbleLauncher {
		if err := launcher.OpenBubbleLauncher(tconn)(ctx); err != nil {
			s.Fatal("Failed to open bubble launcher: ", err)
		}
	} else {
		if err := launcher.OpenExpandedView(tconn)(ctx); err != nil {
			s.Fatal("Failed to open Expanded Application list view: ", err)
		}
	}

	if err := launcher.WaitForStableNumberOfApps(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for item count in app list to stabilize: ", err)
	}

	if err := launcher.CreateFolder(ctx, tconn, productivityLauncher); err != nil {
		s.Fatal("Failed to create folder app: ", err)
	}

	// Add 5 app items to the folder.
	if err := launcher.AddItemsToFolder(ctx, tconn, launcher.UnnamedFolderFinder, 5, !usingBubbleLauncher); err != nil {
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
		if err := launcher.RemoveIconFromFolder(tconn, launcher.UnnamedFolderFinder)(ctx); err != nil {
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
		if err := launcher.RemoveIconFromFolder(tconn, launcher.UnnamedFolderFinder)(ctx); err != nil {
			s.Fatal("Failed to remove icon from folder: ", err)
		}
	}

	// With productivity launcher enabled, launcher does not delete single-item folders, so the folder should be around until the last item is dragged out.
	if productivityLauncher {
		if err := launcher.RemoveIconFromFolder(tconn, launcher.UnnamedFolderFinder)(ctx); err != nil {
			s.Fatal("Failed to remove last icon from folder: ", err)
		}
	}

	// Check that there is no longer a folder.
	ui := uiauto.New(tconn)
	if err := ui.WaitUntilGone(launcher.UnnamedFolderFinder)(ctx); err != nil {
		s.Fatal("Folder exists when it should not: ", err)
	}
}
