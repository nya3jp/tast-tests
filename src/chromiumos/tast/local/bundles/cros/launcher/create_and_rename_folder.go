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

func init() {
	testing.AddTest(&testing.Test{
		Func: CreateAndRenameFolder,
		Desc: "Renaming Folder In Launcher",
		Contacts: []string{
			"seewaifu@chromium.org",
			"chromeos-sw-engprod@google.com",
			"tbarzic@chromium.org",
			"cros-system-ui-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{
			{
				Name: "clamshell_mode",
				Val:  false,
			},
			{
				Name:              "tablet_mode",
				Val:               true,
				ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
			},
		},
	})
}

// CreateAndRenameFolder tests if launcher handles renaming of folder correctly.
func CreateAndRenameFolder(ctx context.Context, s *testing.State) {
	tabletMode := s.Param().(bool)

	cr, err := chrome.New(ctx)
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
	if err := launcher.OpenExpandedView(tconn)(ctx); err != nil {
		s.Fatal("Failed to open Expanded Application list view: ", err)
	}

	if err := launcher.CreateFolder(ctx, tconn)(ctx); err != nil {
		s.Fatal("Failed to create folder app: ", err)
	}

	if err := launcher.RenameFolder(tconn, kb, "Unnamed", "NewName")(ctx); err != nil {
		s.Fatal("Failed to rename folder to NewName: ", err)
	}
}
