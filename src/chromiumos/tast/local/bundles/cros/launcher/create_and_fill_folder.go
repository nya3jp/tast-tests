// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"os"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type fillFolderTestCase struct {
	productivityLauncher bool // Whether productivity launcher feature should be enabled
	tabletMode           bool // Whether the test runs in tablet mode
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         CreateAndFillFolder,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Test adding items to a folder in the launcher",
		Contacts: []string{
			"cros-system-ui-eng@google.com",
			"chromeos-sw-engprod@google.com",
			"mmourgos@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      4 * time.Minute,
		Params: []testing.Param{{
			Name: "productivity_launcher_clamshell_mode",
			Val:  fillFolderTestCase{productivityLauncher: true, tabletMode: false},
		}, {
			Name: "clamshell_mode",
			Val:  fillFolderTestCase{productivityLauncher: false, tabletMode: false},
		}, {
			Name:              "productivity_launcher_tablet_mode",
			Val:               fillFolderTestCase{productivityLauncher: true, tabletMode: true},
			ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		}, {
			Name:              "tablet_mode",
			Val:               fillFolderTestCase{productivityLauncher: false, tabletMode: true},
			ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		}},
	})
}

// CreateAndFillFolder tests that a folder can be filled to the maximum allowed size.
func CreateAndFillFolder(ctx context.Context, s *testing.State) {
	// Create 50 fake apps and get the the options to add to the new chrome session.
	opts, extDirBase, err := ash.GetPrepareFakeAppsOptions(50)
	if err != nil {
		s.Fatal("Failed to create 50 fake apps")
	}
	defer os.RemoveAll(extDirBase)

	productivityLauncher := s.Param().(fillFolderTestCase).productivityLauncher
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

	tabletMode := s.Param().(fillFolderTestCase).tabletMode
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, tabletMode)
	if err != nil {
		s.Fatalf("Failed to ensure tablet mode state %t %s: ", tabletMode, err)
	}
	defer cleanup(ctx)

	if !tabletMode {
		if err := ash.WaitForLauncherState(ctx, tconn, ash.Closed); err != nil {
			s.Fatal("Launcher not closed after transition to clamshell mode: ", err)
		}
	}

	// Open the Launcher on the apps grid page.
	if productivityLauncher && !tabletMode {
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

	paginatedAppList := !productivityLauncher || tabletMode
	// The folder already has 2 items. Add 46 more items to get to the maximum folder size of 48 apps.
	if err := launcher.AddItemsToFolder(ctx, tconn, launcher.UnnamedFolderFinder, 46, paginatedAppList); err != nil {
		s.Fatal("Failed to add items to folder: ", err)
	}

	// Check that the number of apps in the folder is 48.
	size, err := launcher.GetFolderSize(ctx, tconn, launcher.UnnamedFolderFinder)
	if err != nil {
		s.Fatal("Failed to get the folder size: ", err)
	}
	if size != 48 {
		s.Fatalf("Unexpected number of items in folder, got %d, want %d", size, 48)
	}

	// Attempt to add one more item to the folder.
	if err := launcher.AddItemsToFolder(ctx, tconn, launcher.UnnamedFolderFinder, 1, paginatedAppList); err != nil {
		s.Fatal("Failed to add items to folder: ", err)
	}

	// Because the folder was already filled to the maximum size, the number of apps in the folder should still be 48.
	// Check that the folder size remains at the max of 48.
	size, err = launcher.GetFolderSize(ctx, tconn, launcher.UnnamedFolderFinder)
	if err != nil {
		s.Fatal("Failed to get the folder size: ", err)
	}
	if size != 48 {
		s.Fatalf("Unexpected number of items in folder, got %d, want %d", size, 48)
	}
}
