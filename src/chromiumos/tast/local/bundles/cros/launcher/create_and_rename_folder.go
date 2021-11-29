// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CreateAndRenameFolder,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Renaming Folder In Launcher",
		Contacts: []string{
			"seewaifu@chromium.org",
			"chromeos-sw-engprod@google.com",
			"tbarzic@chromium.org",
			"cros-system-ui-eng@google.com",
		},
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

// CreateAndRenameFolder tests if launcher handles renaming of folder correctly.
func CreateAndRenameFolder(ctx context.Context, s *testing.State) {
	testCase := s.Param().(launcher.TestCase)
	tabletMode := testCase.TabletMode

	productivityLauncher := testCase.ProductivityLauncher
	var opt chrome.Option
	if productivityLauncher {
		opt = chrome.EnableFeatures("ProductivityLauncher")
	} else {
		opt = chrome.DisableFeatures("ProductivityLauncher")
	}

	cr, err := chrome.New(ctx, opt)
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	originallyEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to check if DUT is in tablet mode: ", err)
	}
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, tabletMode)
	if err != nil {
		s.Fatal("Failed to ensure clamshell/tablet mode: ", err)
	}
	defer cleanup(ctx)

	// When a DUT switches from tablet mode to clamshell mode, sometimes it takes a while to settle down.
	if originallyEnabled && !tabletMode {
		if err := ash.WaitForLauncherState(ctx, tconn, ash.Closed); err != nil {
			s.Fatal("Launcher not closed after transition to clamshell mode: ", err)
		}
	}

	// Open the Launcher and go to Apps list page.
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

	if err := launcher.RenameFolder(tconn, kb, launcher.UnnamedFolderFinder.First(), "NewName")(ctx); err != nil {
		s.Fatal("Failed to rename folder to NewName: ", err)
	}
}
