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
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CreateAndFillFolder,
		LacrosStatus: testing.LacrosVariantUnneeded,
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
			Val:  launcher.TestCase{TabletMode: false},
		}, {
			Name:              "productivity_launcher_tablet_mode",
			Val:               launcher.TestCase{TabletMode: true},
			ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		}},
	})
}

// CreateAndFillFolder tests that a folder can be filled to the maximum allowed size.
func CreateAndFillFolder(ctx context.Context, s *testing.State) {
	extDirBase, err := ioutil.TempDir("", "")
	if err != nil {
		s.Fatal("Failed to create a temporary directory: ", err)
	}
	defer os.RemoveAll(extDirBase)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Create 50 fake apps and get the the options to add to the new chrome session.
	opts, err := ash.GeneratePrepareFakeAppsOptions(extDirBase, 50)
	if err != nil {
		s.Fatal("Failed to create 50 fake apps")
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

	// The folder already has 2 items. Add 46 more items to get to the maximum folder size of 48 apps.
	if err := launcher.AddItemsToFolder(ctx, tconn, launcher.UnnamedFolderFinder, 46, tabletMode); err != nil {
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
	if err := launcher.AddItemsToFolder(ctx, tconn, launcher.UnnamedFolderFinder, 1, tabletMode); err != nil {
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
