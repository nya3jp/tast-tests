// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type renameFolderTestCase struct {
	productivityLauncher bool	// Whether productivity launcher feature should be enabled
	tabletMode bool			// Whether the test runs in tablet mode
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         CreateAndRenameFolder,
		LacrosStatus: testing.LacrosVariantUnknown,
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
			Val: renameFolderTestCase{productivityLauncher :true, tabletMode: false},
		}, {
			Name: "clamshell_mode",
			Val: renameFolderTestCase{productivityLauncher: false, tabletMode: false},
		}, {
			Name: "productivity_launcher_tablet_mode",
			Val: renameFolderTestCase{productivityLauncher :true, tabletMode: true},
			ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		}, {
			Name: "tablet_mode",
			Val: renameFolderTestCase{productivityLauncher: false, tabletMode: true},
			ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		}},
	})
}

// CreateAndRenameFolder tests if launcher handles renaming of folder correctly.
func CreateAndRenameFolder(ctx context.Context, s *testing.State) {
	tabletMode := s.Param().(renameFolderTestCase).tabletMode

	productivityLauncher := s.Param().(renameFolderTestCase).productivityLauncher
	opts := make([]chrome.Option, 0, 1)
	if productivityLauncher {
		opts = append(opts, chrome.EnableFeatures("ProductivityLauncher"))
	} else {
		opts = append(opts, chrome.DisableFeatures("ProductivityLauncher"))
	}

	cr, err := chrome.New(ctx, opts...)
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

	ui := uiauto.New(tconn)

	// When a DUT switches from tablet mode to clamshell mode, sometimes it takes a while to settle down.
	// Tablet mode's home screen has application icons.
	// On the other hand, clamshell mode's home screen does not have any application icons and users
	// have to expand the launcher to see application icons.
	// Therefore, the following code waits for the icons to go away when changing from tablet mode
	// to clamshell mode.
	if originallyEnabled && !tabletMode {
		launcherNode := nodewith.ClassName(launcher.ExpandedItemsClass)
		if err := ui.WaitUntilGone(launcherNode)(ctx); err != nil {
			s.Fatal("Failed to wait tablet mode to clamshell mode transition complete: ", err)
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

	if err := launcher.RenameFolder(tconn, kb, "Unnamed", "NewName")(ctx); err != nil {
		s.Fatal("Failed to rename folder to NewName: ", err)
	}
}
