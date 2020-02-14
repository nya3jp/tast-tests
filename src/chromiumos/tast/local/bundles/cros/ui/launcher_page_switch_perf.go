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
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/metrics"
	chromeui "chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LauncherPageSwitchPerf,
		Desc:         "Measures smoothness of switching pages within the launcher",
		Contacts:     []string{"mukai@chromium.org", "andrewxu@chromium.org", "cros-launcher-prod-notifications@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Pre:          ash.LoggedInWith100DummyApps(),
		Params: []testing.Param{
			testing.Param{
				Name: "clamshell_mode",
				Val:  false,
			},
			testing.Param{
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

	connected, err := display.PhysicalDisplayConnected(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to check physical display existence: ", err)
	}

	inTabletMode := s.Param().(bool)

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, inTabletMode)
	if err != nil {
		s.Fatal("Failed to ensure in clamshell mode: ", err)
	}
	defer cleanup(ctx)

	// The functions to conduct the user operations. Mouse is used for the
	// clamshell mode.
	clickFunc := func(ctx context.Context, obj *chromeui.Node) error {
		return obj.LeftClick(ctx)
	}
	dragFunc := func(ctx context.Context, start, end ash.Location) error {
		return ash.MouseDrag(ctx, tconn, start, end, dragDuration)
	}
	if inTabletMode {
		// In tablet mode, operations are done through the touch screen.
		tew, err := input.Touchscreen(ctx)
		if err != nil {
			s.Fatal("Failed to get access to the touch screen: ", err)
		}
		defer tew.Close()
		orientation, err := display.GetOrientation(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to obtain the orientation info: ", err)
		}
		tew.SetRotation(-orientation.Angle)

		tcc, err := ash.NewTouchCoordConverter(ctx, tconn, tew)
		if err != nil {
			s.Fatal("Failed to create touch coord converter: ", err)
		}
		stw, err := tew.NewSingleTouchWriter()
		if err != nil {
			s.Fatal("Failed to create touch coord converter: ", err)
		}
		defer stw.Close()
		clickFunc = func(ctx context.Context, obj *chromeui.Node) error {
			rect := ash.Rect{
				Left: obj.Location.Left, Top: obj.Location.Top,
				Width: obj.Location.Width, Height: obj.Location.Height}
			x, y := tcc.ConvertLocation(rect.CenterPoint())
			defer stw.End()
			return stw.Move(x, y)
		}
		dragFunc = func(ctx context.Context, start, end ash.Location) error {
			startX, startY := tcc.ConvertLocation(start)
			endX, endY := tcc.ConvertLocation(end)
			defer stw.End()
			return stw.Swipe(ctx, startX, startY, endX, endY, dragDuration)
		}
	}

	if conns, err := ash.CreateWindows(ctx, cr, ui.PerftestURL, 2); err != nil {
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

	// Find the apps grid view bounds.
	root, err := chromeui.Root(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the root: ", err)
	}
	appsGridView, err := root.DescendantWithTimeout(ctx, chromeui.FindParams{ClassName: "AppsGridView"}, time.Second)
	if err != nil {
		s.Fatal("Failed to find the apps-grid: ", err)
	}
	pageSwitcher, err := root.Descendant(ctx, chromeui.FindParams{ClassName: "PageSwitcher"})
	if err != nil {
		s.Fatal("Failed to find the page switcher of the app-list: ", err)
	}
	pageButtons, err := pageSwitcher.Descendants(ctx, chromeui.FindParams{ClassName: "Button"})
	if err != nil {
		s.Fatal("Failed to find the page buttons: ", err)
	} else if len(pageButtons) < 3 {
		s.Fatalf("There are too few pages (%d), want more than 2 pages", len(pageButtons))
	}

	suffix := "ClamshellMode"
	if inTabletMode {
		suffix = "TabletMode"
	}

	pv := perf.NewValues()

	// First: scroll by click. Clicking the second one, clicking the first one to
	// go back, clicking the last one to long-jump, clicking the first one again
	// to long-jump back to the original page.
	hists, err := metrics.Run(ctx, cr, func() error {
		for step, idx := range []int{1, 0, len(pageButtons) - 1, 0} {
			if err := clickFunc(ctx, pageButtons[idx]); err != nil {
				return errors.Wrapf(err, "failed to click %d-th button (at step %d)", idx, step)
			}
			if err := testing.Sleep(ctx, time.Second); err != nil {
				return errors.Wrap(err, "failed to wait")
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
			s.Fatal("Failed to find the histogram data: ", err)
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
	// launcher itself), this scenario conducts next-page scrolling twice so that
	// the scrolling to the previous page won't arrive to the first page.
	// Currently, the drag operations are not conducted unless there's a physical
	// display because the metrics rely on the presentation callback from the
	// display.
	if connected {
		appsGridLocation := appsGridView.Location
		// drag-up gesture positions; starting at the bottom of the apps-grid (but
		// not at the edge), and moves the height of the apps-grid. The X position
		// should not be the center of the width since it would fall into an app
		// icon. For now, it just sets 2/5 width position to avoid app icons.
		dragUpStart := ash.Location{
			X: appsGridLocation.Left + appsGridLocation.Width*2/5,
			Y: appsGridLocation.Top + appsGridLocation.Height - 1}
		dragUpEnd := ash.Location{X: dragUpStart.X, Y: appsGridLocation.Top - 1}

		// drag-down gesture positions; starting at the top of the apps-grid (but
		// not at the edge), and moves to the bottom edge of the apps-grid. Same
		// X position as the drag-up gesture.
		dragDownStart := ash.Location{X: dragUpStart.X, Y: appsGridLocation.Top + 1}
		dragDownEnd := ash.Location{X: dragDownStart.X, Y: dragDownStart.Y + appsGridLocation.Height}

		hists, err = metrics.Run(ctx, cr, func() error {
			// First drag-up operation.
			if err := dragFunc(ctx, dragUpStart, dragUpEnd); err != nil {
				return errors.Wrap(err, "failed to drag from the bottom to the top")
			}
			if err := testing.Sleep(ctx, time.Second); err != nil {
				return errors.Wrap(err, "failed to wait")
			}
			// Second drag-up operation.
			if err := dragFunc(ctx, dragUpStart, dragUpEnd); err != nil {
				return errors.Wrap(err, "failed to drag from the bottom to the top")
			}
			if err := testing.Sleep(ctx, time.Second); err != nil {
				return errors.Wrap(err, "failed to wait")
			}
			// drag-down operation.
			if err := dragFunc(ctx, dragDownStart, dragDownEnd); err != nil {
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
				s.Fatal("Failed to find the histogram data: ", err)
			}
			pv.Set(perf.Metric{
				Name:      hist.Name,
				Unit:      "ms",
				Direction: perf.SmallerIsBetter,
			}, mean)
		}
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Fatal("Failed saving perf data: ", err)
	}
}
