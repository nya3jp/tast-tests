// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"io/ioutil"
	"os"
	"time"

	"chromiumos/tast/ctxutil"
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
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test removing items from a folder in the launcher",
		Contacts: []string{
			"cros-system-ui-eng@google.com",
			"chromeos-sw-engprod@google.com",
			"mmourgos@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "productivity_launcher_clamshell_mode",
			Val:  launcher.TestCase{TabletMode: false},
		}, {
			Name:              "productivity_launcher_tablet_mode",
			Val:               launcher.TestCase{TabletMode: true},
			ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		}},
	})
}

// RemoveAppsFromFolder tests that items can be removed from a folder.
func RemoveAppsFromFolder(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	extDirBase, err := ioutil.TempDir("", "")
	if err != nil {
		s.Fatal("Failed to create a temporary directory: ", err)
	}
	defer os.RemoveAll(extDirBase)

	// Create 10 fake apps and get the the options to add to the new chrome session.
	opts, err := ash.GeneratePrepareFakeAppsOptions(extDirBase, 10)
	if err != nil {
		s.Fatal("Failed to create 10 fake apps")
	}

	// Creating fake apps and logging into a new session in this test ensures that enough apps will be available to folder.
	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	testCase := s.Param().(launcher.TestCase)
	tabletMode := testCase.TabletMode

	cleanup, err := launcher.SetUpLauncherTest(ctx, tconn, tabletMode, true /*productivityLauncher*/, true /*stabilizeAppCount*/)
	if err != nil {
		s.Fatal("Failed to set up launcher test case: ", err)
	}
	defer cleanup(cleanupCtx)

	if err := launcher.CreateFolder(ctx, tconn, true /*productivityLauncher*/); err != nil {
		s.Fatal("Failed to create folder app: ", err)
	}

	// Add 5 app items to the folder.
	if err := launcher.AddItemsToFolder(ctx, tconn, launcher.UnnamedFolderFinder, 5, tabletMode); err != nil {
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

	// Launcher does not delete single-item folders, so the folder should be around until the last item is dragged out.
	if err := launcher.RemoveIconFromFolder(tconn, launcher.UnnamedFolderFinder)(ctx); err != nil {
		s.Fatal("Failed to remove last icon from folder: ", err)
	}

	// Check that there is no longer a folder.
	ui := uiauto.New(tconn)
	if err := ui.WaitUntilGone(launcher.UnnamedFolderFinder)(ctx); err != nil {
		s.Fatal("Folder exists when it should not: ", err)
	}
}
