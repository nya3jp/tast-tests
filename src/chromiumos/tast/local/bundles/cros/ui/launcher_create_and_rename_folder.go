// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/launcher"
	"chromiumos/tast/local/chrome/ui/pointer"
	"chromiumos/tast/local/input"
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
		Params: []testing.Param{
			{
				Name: "clamshell_mode",
				Val:  false,
			},
			{
				Name:              "tablet_mode",
				Val:               true,
				ExtraSoftwareDeps: []string{"tablet_mode"},
			},
		},
	})
}

// LauncherCreateAndRenameFolder tests if launcher handles renaming of folder correctly.
func LauncherCreateAndRenameFolder(ctx context.Context, s *testing.State) {
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
	// Tablet mode's home screen has application icons.
	// On the other hand, clamshell mode's home screen does not have any application icons and users
	// have to expand the launcher to see application icons.
	// Therefore, the following code waits for the icons to go away when changing from tablet mode
	// to clamshell mode.
	if originallyEnabled && !tabletMode {
		params := ui.FindParams{ClassName: launcher.ExpandedItemsClass}
		if err := ui.WaitUntilGone(ctx, tconn, params, 10*time.Second); err != nil {
			s.Fatal("Failed to wait tablet mode to clamshell mode transition complete: ", err)
		}
	}

	// Open the Launcher and go to Apps list page.
	if err := launcher.OpenExpandedView(ctx, tconn); err != nil {
		s.Fatal("Failed to open Expanded Application list view: ", err)
	}

	if err := ui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
		s.Fatal("Filed to wait for location changes: ", err)
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

	// Drag one icon to the top of another icon to create a folder.
	if err := dragIcon(ctx, tconn, icons[0], icons[1], tabletMode); err != nil {
		s.Fatalf("Failed to Drag app %q: %v", icons[0].Name, err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	if err := launcher.RenameFolder(ctx, tconn, "Unnamed", "NewName", kb); err != nil {
		s.Fatal("Failed to rename folder to NewName: ", err)
	}

}

// dragIcon is a helper function to cause a drag of the left button from start to end for tablet.
func dragIcon(ctx context.Context, tconn *chrome.TestConn, srcIcon, targetIcon *ui.Node, tabletMode bool) error {
	start := srcIcon.Location.CenterPoint()
	end := targetIcon.Location.CenterPoint()

	// First make sure the icon is in focus first to make the test more stable.
	// If the icon is not in focus, the drag and drop operations fail frequently.
	if err := srcIcon.FocusAndWait(ctx, 2*time.Second); err != nil {
		return errors.Wrapf(err, "failed to focus on icon %q", srcIcon.Name)
	}

	var ctlr pointer.Controller
	var err error
	if tabletMode {
		// Setting up touch control
		ctlr, err = pointer.NewTouchController(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to create the touch controller")
		}
	} else {
		// Setting up mouse control
		ctlr = pointer.NewMouseController(tconn)
	}
	// Simulate a long press so that the icon is ready to be moved.
	// It is based on the implementation on ui.LongPress, but this
	// implementation works for both touch screen and mouse press.
	// The function ui.LongPress only works for touch screen.
	// The wait time should be longer than
	// chrome's default long press wait time, which is 500ms.
	if err = ctlr.Press(ctx, start); err != nil {
		return errors.Wrap(err, "failed to click icon")
	}
	if err = testing.Sleep(ctx, 1*time.Second); err != nil {
		return errors.Wrap(err, "failed to long press")
	}
	// Move from one icon to the top of another icon.
	if err = ctlr.Move(ctx, start, end, time.Second); err != nil {
		return errors.Wrap(err, "failed to drag icon")
	}

	// The following statement makes sure the icon was moved to the top of another one
	// in order to prevent the early release of the mouse.
	if err := ui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to wait for location changes")
	}

	// Make sure we hold long enough to let the merge animation starts.
	if err = ctlr.Release(ctx); err != nil {
		return errors.Wrap(err, "failed to drop icon to target")
	}

	// When an icon is dragged and dropped onto another icon, the two icons will
	// become a single folder icon.
	// The following code make sure the rearrangement of icons are done because
	// the next step will click on the folder to rename the folder.
	// We want to make sure we would click on the correct location of the icon.
	if err := ui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to wait for location changes")
	}

	return nil
}
