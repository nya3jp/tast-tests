// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/pointer"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/launcher"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

// init adds the test LauncherCreateAndRenameFolder.
func init() {
	testing.AddTest(&testing.Test{
		Func: LauncherCreateAndRenameFolder,
		Desc: "Renaming Folder In Launcher",
		Contacts: []string{
			"seewaifu@chromium.org",
			"kyleshima@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

// LauncherCreateAndRenameFolder tests if launcher handles renaming of folder correctly.
func LauncherCreateAndRenameFolder(ctx context.Context, s *testing.State) {
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

	tabletMode, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to check if it is in tablet mode: ", err)
	}

	// Open the Launcher and go to Apps list page.
	if err := launcher.OpenExpandedView(ctx, tconn); err != nil {
		s.Fatal("Failed to open Expanded Application list view: ", err)
	}

	// Create a folder in launcher by dragging one app on top of another.
	params := ui.FindParams{ClassName: launcher.ExpandedItemsClass}
	icons, err := ui.FindAll(ctx, tconn, params)
	if err != nil {
		s.Fatal("Failed to find all in expanded launcher: ", err)
	}
	defer icons.Release(ctx)
	if len(icons) < 2 {
		s.Fatal("Not enough icons in expanded launcher to perform test")
	}
	point0 := icons[0].Location.CenterPoint()
	point1 := icons[1].Location.CenterPoint()

	if err := ui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for location change after dragging one icon to another one: ", err)
	}
	// Drag one app on top of another.

	if tabletMode {
		if err := dragIconInTabletMode(ctx, tconn, point0, point1); err != nil {
			s.Fatalf("Failed to Drag app %q: %v", icons[0].Name, err)
		}
	} else {
		if err := dragIconInClamshellMode(ctx, tconn, point0, point1); err != nil {
			s.Fatalf("Failed to Drag app %q: %v", icons[0].Name, err)
		}
	}

	if err := ui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for location change after dragging one icon to another one: ", err)
	}

	if err := launcher.RenameFolder(ctx, tconn, "Unnamed", "NewName"); err != nil {
		s.Fatal("Failed to rename folder to NewName: ", err)
	}

}

// dragIconInTabletMode is a helper function to cause a drag of the left button from start to end for tablet.
func dragIconInTabletMode(ctx context.Context, tconn *chrome.TestConn, start, end coords.Point) error {
	// Setting up touch control.
	tc, err := pointer.NewTouchController(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to create the touch controller")
	}
	stw := tc.EventWriter()
	tcc := tc.TouchCoordConverter()
	defer tc.Close()

	startX, startY := tcc.ConvertLocation(start)
	endX, endY := tcc.ConvertLocation(end)

	// Hold long enough so that the icon is ready to be moved.
	if err := stw.LongPressAt(ctx, startX, startY); err != nil {
		return errors.Wrap(err, "failed to long press")
	}

	// Move from one icon to another icon.
	if err := stw.Swipe(ctx, startX, startY, endX, endY, 2*time.Second); err != nil {
		return errors.Wrap(err, "failed to swipe")
	}

	// Make sure we hold long enough to let the merge animation starts.
	if err := stw.LongPressAt(ctx, endX, endY); err != nil {
		return errors.Wrap(err, "failed to long press")
	}

	return stw.End()
}

// dragIconInClamshellMode is a helper function to cause a drag of the left button from start to end for clamshell chromebook.
func dragIconInClamshellMode(ctx context.Context, tconn *chrome.TestConn, start, end coords.Point) error {
	// Sometimes mouse drag does not wait long enough for each substep.
	// Introduce a delay to make sure UI action is done before moving on to next step.
	if err := mouse.Move(ctx, tconn, start, 0); err != nil {
		return errors.Wrap(err, "failed to move to the start location")
	}
	if err := ui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to wait for location changes")
	}
	if err := mouse.Press(ctx, tconn, mouse.LeftButton); err != nil {
		return errors.Wrap(err, "failed to press the button")
	}
	if err := ui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to wait for location changes")
	}
	if err := mouse.Move(ctx, tconn, end, time.Second); err != nil {
		return errors.Wrap(err, "failed to drag")
	}
	if err := ui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to wait for location changes")
	}
	return mouse.Release(ctx, tconn, mouse.LeftButton)
}
