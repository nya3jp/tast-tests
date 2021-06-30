// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/coords"
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
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{
			{
				Name: "clamshell_mode",
				Val:  false,
			},
			{
				Name:              "tablet_mode",
				Val:               true,
				ExtraSoftwareDeps: []string{"tablet_mode"},
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

	// Drag one icon to the top of another icon to create a folder.
	if err := createFolder(ctx, tconn, ui); err != nil {
		s.Fatal("Failed to create folder app: ", err)
	}

	if err := launcher.RenameFolder(tconn, kb, "Unnamed", "NewName")(ctx); err != nil {
		s.Fatal("Failed to rename folder to NewName: ", err)
	}
}

// createFolder is a helper function to create a folder by dragging the first icon on top of the second icon.
func createFolder(ctx context.Context, tconn *chrome.TestConn, ui *uiauto.Context) error {
	// Create a folder in launcher by dragging the first icon on top of the second icon.
	folder := nodewith.Name("Folder Unnamed").ClassName(launcher.ExpandedItemsClass)

	return ui.Retry(3, uiauto.Combine("createFolder",
		dragIconToIcon(ctx, tconn, ui),
		ui.WaitUntilExists(folder)))(ctx)
}

// dragIconToIcon drags from one icon to another icon.
// We cannot use simple mouse.Drag because this CL https://crrev.com/c/2937656 changes the default UI behavior
// which causes the location of icons after a mouse press on an icon.
// This function will delay the calculation of the coordation of the destination until after mouse press.
func dragIconToIcon(pctx context.Context, tconn *chrome.TestConn, ui *uiauto.Context) func(ctx context.Context) error {
	const duration = time.Second * 2
	return func(ctx context.Context) error {
		src := nodewith.ClassName(launcher.ExpandedItemsClass).First()
		start, err := ui.Location(ctx, src)
		if err != nil {
			return errors.Wrap(err, "failed to get locaton for first icon")
		}
		if err := mouse.Move(tconn, start.CenterPoint(), 0)(ctx); err != nil {
			return errors.Wrap(err, "failed to move to the start location")
		}
		if err := mouse.Press(tconn, mouse.LeftButton)(ctx); err != nil {
			return errors.Wrap(err, "failed to press the button")
		}

		// get destination location after mouse press.
		dest := nodewith.ClassName(launcher.ExpandedItemsClass).Nth(1)
		end, err := ui.Location(ctx, dest)
		if err != nil {
			return errors.Wrap(err, "failed to get locaton for second icon")
		}
		// Move a little bit first to trigger launcher-app-paging.
		if err := mouse.Move(tconn, start.CenterPoint().Add(coords.Point{X: 1, Y: 1}), duration)(ctx); err != nil {
			return errors.Wrap(err, "failed to drag")
		}

		// Get destination location again after auncher-app-paging is done.
		dest = nodewith.ClassName(launcher.ExpandedItemsClass).Nth(1)
		end, err = ui.Location(ctx, dest)
		if err != nil {
			return errors.Wrap(err, "failed to get locaton for second icon")
		}
		if err := mouse.Move(tconn, end.CenterPoint(), duration)(ctx); err != nil {
			return errors.Wrap(err, "failed to drag")
		}
		return mouse.Release(tconn, mouse.LeftButton)(ctx)
	}
}
