// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/pointer"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/metrics"
	chromeui "chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/media/cpu"
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
		Pre:          ash.LoggedInWith100DummyApps(),
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
	const dragDuration = 2 * time.Second
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	defer chromeui.WaitForLocationChangeCompleted(ctx, tconn)

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

	if conns, err := ash.CreateWindows(ctx, tconn, cr, ui.PerftestURL, 2); err != nil {
		s.Fatal("Failed to create windows: ", err)
	} else {
		// unnecessary to use the connections; simply close now.
		conns.Close()
	}
	if !inTabletMode {
		// In clamshell mode, turn all windows into normal state, so the desktop
		// under the app-launcher has the combination of window and wallpaper. This
		// is not the case of the tablet mode (since windows are always maximized in
		// the tablet mode).
		ws, err := ash.GetAllWindows(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get the window list: ", err)
		}
		for _, w := range ws {
			if _, err := ash.SetWindowState(ctx, tconn, w.ID, ash.WMEventNormal); err != nil {
				s.Fatalf("Failed to set the window (%d) to normal: %v", w.ID, err)
			}
		}
	}

	// Currently tast test may show a couple of notifications like "sign-in error"
	// and they may overlap with UI components of launcher. This prevents intended
	// actions on certain devices and causes test failures. Open and close the
	// quick settings to dismiss those notification popups. See
	// https://crbug.com/1084185.
	if err := ash.HideAllNotifications(ctx, tconn); err != nil {
		s.Fatal("Failed to open/close the quick settings: ", err)
	}

	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed to wait: ", err)
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
	// Wait for the location changes of launcher UI to be propagated.
	if err := chromeui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for location changes to complete: ", err)
	}

	// Find the apps grid view bounds.
	appsGridView, err := chromeui.FindWithTimeout(ctx, tconn, chromeui.FindParams{ClassName: "AppsGridView"}, time.Second)
	if err != nil {
		s.Fatal("Failed to find the apps-grid: ", err)
	}
	defer appsGridView.Release(ctx)
	pageSwitcher, err := chromeui.Find(ctx, tconn, chromeui.FindParams{ClassName: "PageSwitcher"})
	if err != nil {
		s.Fatal("Failed to find the page switcher of the app-list: ", err)
	}
	defer pageSwitcher.Release(ctx)
	pageButtons, err := pageSwitcher.Descendants(ctx, chromeui.FindParams{ClassName: "Button"})
	if err != nil {
		s.Fatal("Failed to find the page buttons: ", err)
	}
	defer pageButtons.Release(ctx)
	if len(pageButtons) < 3 {
		s.Fatalf("There are too few pages (%d), want more than 2 pages", len(pageButtons))
	}

	suffix := "ClamshellMode"
	if inTabletMode {
		suffix = "TabletMode"
	}

	pv := perf.NewValues()

	const pageSwitchTimeout = 2 * time.Second
	clickPageButtonAndWait := func(ctx context.Context, pageButton *chromeui.Node) error {
		ew, err := chromeui.NewWatcher(ctx, pageButton, chromeui.EventTypeAlert)
		if err != nil {
			return errors.Wrap(err, "failed to create an event watcher")
		}
		defer ew.Release(ctx)
		if err := pointer.Click(ctx, pc, pageButton.Location.CenterPoint()); err != nil {
			return errors.Wrap(err, "failed to click the page button")
		}
		if _, err := ew.WaitForEvent(ctx, pageSwitchTimeout); err != nil {
			return errors.Wrap(err, "failed to wait for the page switch")
		}
		return nil
	}
	// First: scroll by click. Clicking the second one, clicking the first one to
	// go back, clicking the last one to long-jump, clicking the first one again
	// to long-jump back to the original page.
	s.Log("Starting the scroll by click")
	hists, err := metrics.RunAndWaitAll(ctx, tconn, time.Second, func() error {
		for step, idx := range []int{1, 0, len(pageButtons) - 1, 0} {
			if err := clickPageButtonAndWait(ctx, pageButtons[idx]); err != nil {
				return errors.Wrapf(err, "failed to click or wait %d-th button (at step %d)", idx, step)
			}
		}
		return nil
	}, "Apps.PaginationTransition.AnimationSmoothness."+suffix)
	if err != nil {
		s.Fatal("Failed to run the test scenario: ", err)
	}
	for _, hist := range hists {
		mean, err := hist.Mean()
		if err != nil {
			s.Fatalf("Failed to find the histogram data for %s: %v", hist.Name, err)
		}
		pv.Set(perf.Metric{
			Name:      hist.Name,
			Unit:      "percent",
			Direction: perf.BiggerIsBetter,
		}, mean)
	}

	// Second: scroll by drags. This involves two types of operations, drag-up
	// from the bottom for scrolling to the next page, and the drag-down from the
	// top for scrolling to the previous page. In order to prevent overscrolling
	// on the first page which might cause unexpected effects (like closing of the
	// launcher itself), and because the first page might be in a special
	// arrangement of the default apps with blank area at the bottom which
	// prevents the drag-up gesture, switches to the second page before starting
	// this scenario.
	s.Log("Starting the scroll by drags")
	if err := clickPageButtonAndWait(ctx, pageButtons[1]); err != nil {
		s.Fatal("Failed to switch to the second page: ", err)
	}
	// Reset back to the first page at the end of the test; apps-grid page
	// selection may stay in the last page, and that causes troubles on another
	// test. See https://crbug.com/1081285.
	defer pointer.Click(ctx, pc, pageButtons[0].Location.CenterPoint())
	appsGridLocation := appsGridView.Location
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

	hists, err = metrics.RunAndWaitAll(ctx, tconn, time.Second, func() error {
		ew, err := chromeui.NewWatcher(ctx, pageButtons[2], chromeui.EventTypeAlert)
		if err != nil {
			return errors.Wrap(err, "failed to create an event watcher")
		}
		defer ew.Release(ctx)
		// First drag-up operation.
		if err := pointer.Drag(ctx, pc, dragUpStart, dragUpEnd, dragDuration); err != nil {
			return errors.Wrap(err, "failed to drag from the bottom to the top")
		}
		if _, err := ew.WaitForEvent(ctx, pageSwitchTimeout); err != nil {
			// It is actually fine if the drag doesn't cause scrolling to the next
			// page. The required metrics for dragging should be made and enough
			// waiting time should have passed for the next dragging session.
			s.Log("Failed to wait for the page switch; maybe the dragging does not cause the page scroll")
		}
		// drag-down operation.
		if err := pointer.Drag(ctx, pc, dragDownStart, dragDownEnd, dragDuration); err != nil {
			return errors.Wrap(err, "failed to drag from the top to the bottom")
		}
		return nil
	},
		"Apps.PaginationTransition.DragScroll.PresentationTime."+suffix,
		"Apps.PaginationTransition.DragScroll.PresentationTime.MaxLatency."+suffix)
	if err != nil {
		s.Fatal("Failed to run the test scenario: ", err)
	}
	for _, hist := range hists {
		mean, err := hist.Mean()
		if err != nil {
			s.Fatalf("Failed to find the histogram data for %s: %v", hist.Name, err)
		}
		pv.Set(perf.Metric{
			Name:      hist.Name,
			Unit:      "ms",
			Direction: perf.SmallerIsBetter,
		}, mean)
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Fatal("Failed saving perf data: ", err)
	}
}
