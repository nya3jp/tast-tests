// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"sort"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	uiperf "chromiumos/tast/local/bundles/cros/ui/perf"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/perfutil"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type dragType int

const (
	dragTypeNormal dragType = iota
	dragTypeSnap
	dragTypeClose
)

type dragFunc func(ctx context.Context, tsw *input.TouchscreenEventWriter,
	stw *input.SingleTouchEventWriter, tconn *chrome.TestConn) error

type dragTest struct {
	dt dragType // Type of the drag to run.
	l  string   // Label for the metric name.
	df dragFunc // Function to run the drag test.
	bt browser.Type
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         OverviewDragWindowPerf,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Measures the presentation time of window dragging in overview in tablet mode",
		Contacts:     []string{"xiyuan@chromium.org", "mukai@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Timeout:      4 * time.Minute,
		Params: []testing.Param{{
			Name:    "normal_drag",
			Fixture: "chromeLoggedIn",
			Val: dragTest{
				dt: dragTypeNormal,
				l:  "NormalDrag",
				df: normalDrag,
				bt: browser.TypeAsh,
			},
		}, {
			Name:    "drag_to_snap",
			Fixture: "chromeLoggedIn",
			Val: dragTest{
				dt: dragTypeSnap,
				l:  "DragToSnap",
				df: dragToSnap,
				bt: browser.TypeAsh,
			},
		}, {
			Name:    "drag_to_close",
			Fixture: "chromeLoggedIn",
			Val: dragTest{
				dt: dragTypeClose,
				l:  "DragToClose",
				df: dragToClose,
				bt: browser.TypeAsh,
			},
		}, {
			Name:              "normal_drag_lacros",
			Fixture:           "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Val: dragTest{
				dt: dragTypeNormal,
				l:  "NormalDrag",
				df: normalDrag,
				bt: browser.TypeLacros,
			},
		}, {
			Name:              "drag_to_snap_lacros",
			Fixture:           "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Val: dragTest{
				dt: dragTypeSnap,
				l:  "DragToSnap",
				df: dragToSnap,
				bt: browser.TypeLacros,
			},
		}, {
			Name:              "drag_to_close_lacros",
			Fixture:           "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Val: dragTest{
				dt: dragTypeClose,
				l:  "DragToClose",
				df: dragToClose,
				bt: browser.TypeLacros,
			},
		}},
	})
}

func normalDrag(ctx context.Context, tsw *input.TouchscreenEventWriter,
	stw *input.SingleTouchEventWriter, tconn *chrome.TestConn) error {
	x := input.TouchCoord(tsw.Width() / 3)
	y := input.TouchCoord(tsw.Height() / 3)

	// Long press to pick up the overview item at (x, y).
	if err := stw.LongPressAt(ctx, x, y); err != nil {
		return errors.Wrap(err, "failed to long press")
	}

	// Validity check to ensure there is one dragging item.
	_, err := ash.DraggedWindowInOverview(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get dragged overview item")
	}

	// Drag to move it around.
	t := 1 * time.Second
	for _, point := range []struct {
		x, y input.TouchCoord
	}{
		{x + tsw.Width()/2, y},
		{x + tsw.Width()/2, y + tsw.Height()/2},
		{x, y + tsw.Height()/2},
		{x, y},
	} {
		if err := stw.Swipe(ctx, x, y, point.x, point.y, t); err != nil {
			return errors.Wrap(err, "failed to swipe")
		}

		x = point.x
		y = point.y
	}

	return stw.End()
}

// leftOrTop returns the coordinates of left or top position based on screen
// orientation and the given coordinates.
func leftOrTop(ctx context.Context, tconn *chrome.TestConn, x, y input.TouchCoord) (input.TouchCoord, input.TouchCoord, error) {
	orientation, err := display.GetOrientation(ctx, tconn)
	if err != nil {
		return x, y, errors.Wrap(err, "failed to get screen orientation")
	}
	var tx, ty input.TouchCoord
	switch orientation.Type {
	case display.OrientationLandscapePrimary:
		tx = input.TouchCoord(0)
		ty = y
	case display.OrientationPortraitPrimary:
		tx = x
		ty = input.TouchCoord(0)
	default:
		return x, y, errors.New("unsupported screen orientation")
	}

	return tx, ty, nil
}

func dragToSnap(ctx context.Context, tsw *input.TouchscreenEventWriter,
	stw *input.SingleTouchEventWriter, tconn *chrome.TestConn) error {
	x := input.TouchCoord(tsw.Width() / 3)
	y := input.TouchCoord(tsw.Height() / 3)

	// Long press to pick up the overview item at (x, y).
	if err := stw.LongPressAt(ctx, x, y); err != nil {
		return errors.Wrap(err, "failed to long press")
	}

	// Validity check to ensure there is one dragging item.
	_, err := ash.DraggedWindowInOverview(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get dragged overview item")
	}

	// Get left or top target position to drag to.
	tx, ty, err := leftOrTop(ctx, tconn, x, y)
	if err != nil {
		return err
	}

	// Drag to the left or top edge to snap it.
	if err := stw.Swipe(ctx, x, y, tx, ty, 750*time.Millisecond); err != nil {
		return errors.Wrap(err, "failed to swipe")
	}

	if err := stw.End(); err != nil {
		return errors.Wrap(err, "failed to release touch")
	}

	// Validity check to ensure one left snapped window.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		snapped, err := ash.SnappedWindows(ctx, tconn)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get snapped windows"))
		}
		if len(snapped) != 1 || snapped[0].State != ash.WindowStateLeftSnapped {
			return errors.New("left snapped window not found")
		}
		return nil
	}, &testing.PollOptions{Timeout: 2 * time.Second}); err != nil {
		return err
	}
	return nil
}

func clearSnap(ctx context.Context, tsw *input.TouchscreenEventWriter,
	stw *input.SingleTouchEventWriter, tconn *chrome.TestConn) error {
	// Checks whether there is a snapped window.
	snapped, err := ash.SnappedWindows(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get snapped windows")
	}
	if len(snapped) == 0 {
		// Do nothing when there is no snapped window.
		return nil
	}

	// Touch down the work area center on the split view divider.
	info, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get internal display info")
	}

	pixelToTouchFactorX := float64(tsw.Width()) / float64(info.Bounds.Width)
	pixelToTouchFactorY := float64(tsw.Height()) / float64(info.Bounds.Height)

	x := input.TouchCoord(float64(info.WorkArea.Width) * pixelToTouchFactorX / 2)
	y := input.TouchCoord(float64(info.WorkArea.Height) * pixelToTouchFactorY / 2)

	if err := stw.Move(x, y); err != nil {
		return errors.Wrap(err, "failed to move touch")
	}

	// Drag to left or top edge to clear the snap.
	tx, ty, err := leftOrTop(ctx, tconn, x, y)
	if err != nil {
		return err
	}

	if err := stw.Swipe(ctx, x, y, tx, ty, 750*time.Millisecond); err != nil {
		return errors.Wrap(err, "failed to swipe")
	}

	if err := stw.End(); err != nil {
		return errors.Wrap(err, "failed to release touch")
	}

	// Validity check that there is no longer snapped windows.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		snapped, err = ash.SnappedWindows(ctx, tconn)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get snapped windows"))
		}
		if len(snapped) != 0 {
			return errors.New("failed to clear snapped window")
		}
		return nil
	}, &testing.PollOptions{Timeout: 2 * time.Second}); err != nil {
		return err
	}
	return nil
}

func dragToClose(ctx context.Context, tsw *input.TouchscreenEventWriter,
	stw *input.SingleTouchEventWriter, tconn *chrome.TestConn) error {
	// Get existing windows before closing.
	oldWindows, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get all windows")
	}

	x := input.TouchCoord(tsw.Width() / 3)
	y := input.TouchCoord(tsw.Height() / 3)

	// No need to long press to pick it up just drag out of screen to close.
	if err := stw.Move(x, y); err != nil {
		return errors.Wrap(err, "failed to move touch")
	}

	if err := stw.Swipe(ctx, x, y, x, tsw.Height()-1, 500*time.Millisecond); err != nil {
		return errors.Wrap(err, "failed to swipe")
	}

	if err := stw.End(); err != nil {
		return errors.Wrap(err, "failed to release touch")
	}

	// Wait for close animation to finish and close the window.
	if err := testing.Sleep(ctx, 500*time.Millisecond); err != nil {
		return errors.Wrap(err, "failed to wait")
	}

	// Validity check that a window is closed.
	newWindows, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get all windows")
	}
	if len(oldWindows) != len(newWindows)+1 {
		return errors.Errorf("window is not closed, got:%d, expect:%d",
			len(newWindows), len(oldWindows)-1)
	}
	return nil
}

func OverviewDragWindowPerf(ctx context.Context, s *testing.State) {
	// Reserve a few seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(cleanupCtx, 10*time.Second)
	defer cancel()

	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		s.Fatal("Failed to ensure in tablet mode: ", err)
	}
	defer cleanup(cleanupCtx)

	// Ensures that the display zoom factor is 90% (allowing for rounding error) or
	// less, to ensure that the work area length is at least twice the minimum length
	// of a browser window, so that browser windows can be snapped in split view.
	const zoomMaximum = 0.90000005
	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the primary display info: ", err)
	}
	if zoomInitial := info.DisplayZoomFactor; zoomInitial > zoomMaximum {
		i := sort.Search(len(info.AvailableDisplayZoomFactors), func(i int) bool { return info.AvailableDisplayZoomFactors[i] > zoomMaximum })
		if i == 0 {
			s.Fatalf("Lowest available display zoom factor is %f; want 90%% (allowing for rounding error) or less", info.AvailableDisplayZoomFactors[0])
		}
		zoomForTest := info.AvailableDisplayZoomFactors[i-1]
		if err := display.SetDisplayProperties(ctx, tconn, info.ID, display.DisplayProperties{DisplayZoomFactor: &zoomForTest}); err != nil {
			s.Fatalf("Failed to set display zoom factor to %f: %v", zoomForTest, err)
		}
		defer display.SetDisplayProperties(cleanupCtx, tconn, info.ID, display.DisplayProperties{DisplayZoomFactor: &zoomInitial})
	}

	tsw, err := input.Touchscreen(ctx)
	if err != nil {
		s.Fatal("Failed to open touchscreen device: ", err)
	}
	defer tsw.Close()
	orientation, err := display.GetOrientation(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the display orientation: ", err)
	}
	if err = tsw.SetRotation(-orientation.Angle); err != nil {
		s.Fatal("Failed to set rotation: ", err)
	}

	stw, err := tsw.NewSingleTouchWriter()
	if err != nil {
		s.Fatal("Failed to create single touch writer: ", err)
	}
	defer stw.Close()

	const histName = "Ash.Overview.WindowDrag.PresentationTime.TabletMode"

	// Note that the test needs to take traces in ash-chrome, and grab the metrics from ash-chrome.
	// So, ash-chrome `cr` should be used for perfutil.NewRunner and ash test APIs `tconn` for RunAndWaitAll here in this test.
	runner := perfutil.NewRunner(cr.Browser())
	drag := s.Param().(dragTest)

	defer ash.SetOverviewModeAndWait(ctx, tconn, false)
	var br *browser.Browser
	const url = ui.PerftestURL
	currentWindows := 0
	// Run the test cases with different number of browser windows.
	for _, windows := range []int{2, 8} {
		// Open a first window using browserfixt to get a Browser instance, then use the browser instance for other windows.
		if currentWindows == 0 {
			var conn *browser.Conn
			var closeBrowser func(ctx context.Context) error
			conn, br, closeBrowser, err = browserfixt.SetUpWithURL(ctx, cr, drag.bt, url)
			if err != nil {
				s.Fatal("Failed to open chrome: ", err)
			}
			defer closeBrowser(cleanupCtx)
			defer conn.Close()
			currentWindows++
		}
		if err := ash.CreateWindows(ctx, tconn, br, url, windows-currentWindows); err != nil {
			s.Fatal("Failed to open windows: ", err)
		}
		currentWindows = windows

		if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
			s.Fatal("Failed to enter into the overview mode: ", err)
		}

		// Run the drag and collect histogram.
		suffix := fmt.Sprintf("%dwindows", currentWindows)
		runner.RunMultiple(ctx, suffix, uiperf.Run(s, perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
			if err := drag.df(ctx, tsw, stw, tconn); err != nil {
				return errors.Wrap(err, "failed to run drag")
			}

			// Clean up.
			switch drag.dt {
			case dragTypeSnap:
				if err := clearSnap(ctx, tsw, stw, tconn); err != nil {
					s.Fatal("Failed to clearSnap: ", err)
				}
			case dragTypeClose:
				if err := ash.CreateWindows(ctx, tconn, br, url, 1); err != nil {
					return errors.Wrap(err, "failed to create windows")
				}
				if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
					return errors.Wrap(err, "failed to re-enter into overview mode")
				}
			}
			return nil
		}, histName)),
			perfutil.StoreAll(perf.SmallerIsBetter, "ms", drag.l+"."+suffix))
	}

	if err := runner.Values().Save(ctx, s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
