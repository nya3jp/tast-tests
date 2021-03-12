// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/perfutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui/pointer"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/event"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: LauncherPageSwitchPerf,
		Desc: "Measures smoothness of switching pages within the launcher",
		Contacts: []string{
			"newcomer@chromium.org", "tbarzic@chromium.org", "cros-launcher-prod-notifications@google.com",
			"mukai@chromium.org", // original test author
		},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Fixture:      "chromeLoggedInWith100FakeApps",
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
		Timeout: 3 * time.Minute,
	})
}

func LauncherPageSwitchPerf(ctx context.Context, s *testing.State) {
	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	const dragDuration = 2 * time.Second
	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	inTabletMode := s.Param().(bool)

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, inTabletMode)
	if err != nil {
		s.Fatal("Failed to ensure in clamshell mode: ", err)
	}
	defer cleanup(ctx)

	var pc pointer.Controller
	if inTabletMode {
		var err error
		if pc, err = pointer.NewTouchController(ctx, tconn); err != nil {
			s.Fatal("Failed to create a touch controller")
		}
	} else {
		pc = pointer.NewMouseController(tconn)
	}
	defer pc.Close()

	if err := ash.CreateWindows(ctx, tconn, cr, ui.PerftestURL, 2); err != nil {
		s.Fatal("Failed to create windows: ", err)
	}
	if !inTabletMode {
		// In clamshell mode, turn all windows into normal state, so the desktop
		// under the app-launcher has the combination of window and wallpaper. This
		// is not the case of the tablet mode (since windows are always maximized in
		// the tablet mode).
		if err := ash.ForEachWindow(ctx, tconn, func(w *ash.Window) error {
			return ash.SetWindowStateAndWait(ctx, tconn, w.ID, ash.WindowStateNormal)
		}); err != nil {
			s.Fatal("Failed to set all windows to normal: ", err)
		}
	}

	// Currently tast test may show a couple of notifications like "sign-in error"
	// and they may overlap with UI components of launcher. This prevents intended
	// actions on certain devices and causes test failures. Open and close the
	// quick settings to dismiss those notification popups. See
	// https://crbug.com/1084185.
	if err := ash.HideVisibleNotifications(ctx, tconn); err != nil {
		s.Fatal("Failed to open/close the quick settings: ", err)
	}

	// Search or Shift-Search key to show the apps grid in fullscreen.
	kw, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to obtain the keyboard")
	}
	defer kw.Close()

	accel := "Shift+Search"
	if inTabletMode {
		accel = "Search"
	}
	if err := kw.Accel(ctx, accel); err != nil {
		s.Fatalf("Failed to type %s: %v", accel, err)
	}
	if !inTabletMode {
		// Press the search key to close the app-list at the end. This is not
		// necessary on tablet mode.
		defer kw.Accel(ctx, "Search")
	}

	// Wait for the launcher state change.
	if err := ash.WaitForLauncherState(ctx, tconn, ash.FullscreenAllApps); err != nil {
		s.Fatal("Failed to wait: ", err)
	}

	// Find the apps grid view bounds.
	ac := uiauto.New(tconn)
	appsGridView := nodewith.ClassName("AppsGridView")
	pageButtons := nodewith.ClassName("Button").Ancestor(nodewith.ClassName("PageSwitcher"))

	buttonsInfo, err := ac.NodesInfo(ctx, pageButtons)
	if err != nil {
		s.Fatal("Failed to find the page switcher buttons: ", err)
	}
	if len(buttonsInfo) < 3 {
		s.Fatalf("There are too few pages (%d), want more than 2 pages", len(buttonsInfo))
	}

	suffix := "ClamshellMode"
	if inTabletMode {
		suffix = "TabletMode"
	}

	runner := perfutil.NewRunner(cr)

	const pageSwitchTimeout = 2 * time.Second
	clickPageButtonAndWait := func(ctx context.Context, idx int) error {
		button := pageButtons.Nth(idx)
		return ac.WaitForEvent(button, event.Alert, func(ctx context.Context) error {
			loc, err := ac.Location(ctx, button)
			if err != nil {
				return err
			}
			return pointer.Click(ctx, pc, loc.CenterPoint())
		})(ctx)
	}
	// First: scroll by click. Clicking the second one, clicking the first one to
	// go back, clicking the last one to long-jump, clicking the first one again
	// to long-jump back to the original page.
	s.Log("Starting the scroll by click")
	runner.RunMultiple(ctx, s, "click", perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
		for step, idx := range []int{1, 0, len(buttonsInfo) - 1, 0} {
			if err := clickPageButtonAndWait(ctx, idx); err != nil {
				return errors.Wrapf(err, "failed to click or wait %d-th button (at step %d)", idx, step)
			}
		}
		return nil
	}, "Apps.PaginationTransition.AnimationSmoothness."+suffix),
		perfutil.StoreSmoothness)

	// Second: scroll by drags. This involves two types of operations, drag-up
	// from the bottom for scrolling to the next page, and the drag-down from the
	// top for scrolling to the previous page. In order to prevent overscrolling
	// on the first page which might cause unexpected effects (like closing of the
	// launcher itself), and because the first page might be in a special
	// arrangement of the default apps with blank area at the bottom which
	// prevents the drag-up gesture, switches to the second page before starting
	// this scenario.
	s.Log("Starting the scroll by drags")
	if err := clickPageButtonAndWait(ctx, 1); err != nil {
		s.Fatal("Failed to switch to the second page: ", err)
	}
	// Reset back to the first page at the end of the test; apps-grid page
	// selection may stay in the last page, and that causes troubles on another
	// test. See https://crbug.com/1081285.
	defer func() {
		loc, err := ac.Location(ctx, pageButtons.First())
		if err != nil {
			s.Fatal("Failed to get the location: ", err)
		}
		if err := pointer.Click(ctx, pc, loc.CenterPoint()); err != nil {
			s.Fatal("Failed to click: ", err)
		}
	}()
	appsGridLocation, err := ac.Location(ctx, appsGridView)
	if err != nil {
		s.Fatal("Failed to get the location of appsgridview: ", err)
	}
	// drag-up gesture positions; starting at a bottom part of the apps-grid (at
	// 4/5 height), and moves the height of the apps-grid. The starting height
	// shouldn't be very bottom of the apps-grid, since it may fall into the
	// hotseat (the gesture won't cause page-switch in such case).
	// The X position should not be the center of the width since it would fall
	// into an app icon. For now, it just sets 2/5 width position to avoid app
	// icons.
	dragUpStart := coords.NewPoint(
		appsGridLocation.Left+appsGridLocation.Width*2/5,
		appsGridLocation.Top+appsGridLocation.Height*4/5)
	dragUpEnd := coords.NewPoint(dragUpStart.X, dragUpStart.Y-appsGridLocation.Height)

	// drag-down gesture positions; starting at the top of the apps-grid (but
	// not at the edge), and moves to the bottom edge of the apps-grid. Same
	// X position as the drag-up gesture.
	dragDownStart := coords.NewPoint(dragUpStart.X, appsGridLocation.Top+1)
	dragDownEnd := coords.NewPoint(dragDownStart.X, dragDownStart.Y+appsGridLocation.Height)

	runner.RunMultiple(ctx, s, "drag", perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
		// Drag-up operation.
		action := uiauto.Combine(
			"launcher page drag",
			// Drag-up operation.
			ac.WaitForEvent(pageButtons.Nth(2), event.Alert, func(ctx context.Context) error {
				return pointer.Drag(ctx, pc, dragUpStart, dragUpEnd, dragDuration)
			}),
			// drag-down operation.
			func(ctx context.Context) error {
				return pointer.Drag(ctx, pc, dragDownStart, dragDownEnd, dragDuration)
			},
		)
		return action(ctx)
	},
		"Apps.PaginationTransition.DragScroll.PresentationTime."+suffix,
		"Apps.PaginationTransition.DragScroll.PresentationTime.MaxLatency."+suffix),
		perfutil.StoreLatency)

	if err := runner.Values().Save(ctx, s.OutDir()); err != nil {
		s.Fatal("Failed saving perf data: ", err)
	}
}
