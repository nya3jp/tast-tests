// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/metrics"
	chromeui "chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/pointer"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SplitViewResizePerf,
		Desc:         "Measures smoothness of resizing split view windows",
		Contacts:     []string{"mukai@chromium.org", "sammiequon@chromium.org", "amusbach@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Name: "clamshell_mode",
				Val:  false,
			},
			{
				ExtraSoftwareDeps: []string{"tablet_mode"},
				Pre:               chrome.LoggedIn(),
				Val:               true,
			},
		},
	})
}

func SplitViewResizePerf(ctx context.Context, s *testing.State) {
	// Enables DragToSnapInClamshellMode when testing clamshell split view.
	// TODO(https://crbug.com/1073508): When the feature is fully launched, just
	// use chrome.Pre().
	tabletMode := s.Param().(bool)
	var cr *chrome.Chrome
	var err error
	if tabletMode {
		cr = s.PreValue().(*chrome.Chrome)
	} else {
		cr, err = chrome.New(ctx, chrome.ExtraArgs("--enable-features=DragToSnapInClamshellMode"))
		if err != nil {
			s.Fatal("Failed to connect to Chrome: ", err)
		}
		defer cr.Close(ctx)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, tabletMode)
	if err != nil {
		if tabletMode {
			s.Fatal("Failed to ensure in tablet mode: ", err)
		} else {
			s.Fatal("Failed to ensure in clamshell mode: ", err)
		}
	}
	defer cleanup(ctx)

	// Ensures landscape orientation so this test can assume that windows snap on
	// the left and right. Windows snap on the top and bottom in portrait-oriented
	// tablet mode. They snap on the left and right in portrait-oriented clamshell
	// mode, but there are active (although contentious) proposals to change that.
	orientation, err := display.GetOrientation(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the orientation info: ", err)
	}
	rotation := -orientation.Angle
	if orientation.Type == display.OrientationPortraitPrimary {
		info, err := display.GetPrimaryInfo(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get the primary display info: ", err)
		}
		if err = display.SetDisplayRotationSync(ctx, tconn, info.ID, display.Rotate90); err != nil {
			s.Fatal("Failed to rotate display: ", err)
		}
		defer display.SetDisplayRotationSync(ctx, tconn, info.ID, display.Rotate0)
		rotation += 90
	}

	var pointerController pointer.Controller
	if tabletMode {
		pointerController, err = pointer.NewTouchController(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to create touch controller: ", err)
		}
	} else {
		pointerController = pointer.NewMouseController(tconn)
	}
	defer pointerController.Close()

	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the primary display info: ", err)
	}

	// The following computations assume that the top left corner of the work area
	// is at (0, 0).
	yCenter := info.WorkArea.Height / 2
	// Compute the coordinates where we will drag an overview window to snap left.
	leftSnapPoint := coords.Point{X: 0, Y: yCenter}
	// Compute the coordinates for the actual test scenario: drag the divider
	// slightly left and then all the way right. The left snapped window should
	// shrink and then expand and become maximized.
	dividerDragPointOne := coords.Point{X: info.WorkArea.Width / 2, Y: yCenter}
	dividerDragPointTwo := coords.Point{X: info.WorkArea.Width / 4, Y: yCenter}
	dividerDragPointThree := coords.Point{X: info.WorkArea.Width - 1, Y: yCenter}

	// Testing 3 patterns;
	// SingleWindow: there's a single window which is snapped to the left.
	// WithOverview: one window is snapped to the left, and the right area is
	//   in the overview mode.
	// MultiWindow (tablet only): one window is snapped to the left and another
	//   window is snapped to the right.
	type testCaseSlice []struct {
		name       string
		numWindows int
		customPrep func() error
	}
	var testCases testCaseSlice
	var modeName string
	if tabletMode {
		testCases = testCaseSlice{
			{"SingleWindow", 1, func() error { return nil }},
			{"WithOverview", 8, func() error { return nil }},
			{"MultiWindow", 8, func() error {
				// Additional preparation for the multi-window; by default the right side
				// should be in the overview mode, so here selects one of the windows.
				// First, find the window which is in the overview and obtains the center
				// point of its bounds.
				w, err := ash.FindFirstWindowInOverview(ctx, tconn)
				if err != nil {
					return err
				}
				id1 := w.ID
				if err := pointer.Click(ctx, pointerController, w.OverviewInfo.Bounds.CenterPoint()); err != nil {
					return errors.Wrapf(err, "failed to tap the center of %d", id1)
				}
				return ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
					return w.ID == id1 && !w.IsAnimating && w.State == ash.WindowStateRightSnapped
				}, &testing.PollOptions{Timeout: 5 * time.Second})
			}},
		}
		modeName = "TabletMode"
	} else {
		testCases = testCaseSlice{
			{"SingleWindow", 2, func() error {
				// Additional preparation for the empty overview grid. When we drag and
				// snap an overview item to achieve clamshell split view in the first
				// place, we need a second overview item so that overview stays nonempty;
				// otherwise overview will end. After entering clamshell split view, we
				// need to get rid of that extra overview item, but if we close it then
				// overview will end. So we have to create a second virtual desk and drag
				// the overview item thereto. This workflow may be too rare to warrant
				// performance testing, but the results also apply to the potentially
				// more frequent workflow where one display is in clamshell split view
				// with an empty overview grid while another display has overview items.
				if err := ash.CreateNewDesk(ctx, tconn); err != nil {
					return errors.Wrap(err, "failed to create a new desk")
				}
				w, err := ash.FindFirstWindowInOverview(ctx, tconn)
				if err != nil {
					return errors.Wrap(err, "failed to find the window in the overview mode")
				}
				if err := chromeui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
					return errors.Wrap(err, "failed to wait for location-change events to be propagated to the automation API")
				}
				deskMiniViews, err := chromeui.FindAll(ctx, tconn, chromeui.FindParams{ClassName: "DeskMiniView"})
				if err != nil {
					return errors.Wrap(err, "failed to get desk mini-views")
				}
				defer deskMiniViews.Release(ctx)
				if deskMiniViewCount := len(deskMiniViews); deskMiniViewCount != 2 {
					return errors.Wrapf(err, "expected 2 desk mini-views; found %v", deskMiniViewCount)
				}
				if err := pointer.Drag(ctx, pointerController, w.OverviewInfo.Bounds.CenterPoint(), deskMiniViews[1].Location.CenterPoint(), time.Second); err != nil {
					return errors.Wrap(err, "failed to drag window from overview grid to desk mini-view")
				}
				if _, err := ash.FindFirstWindowInOverview(ctx, tconn); err == nil {
					return errors.New("failed to arrange clamshell split view with empty overview grid")
				}
				return nil
			}},
			// 9 windows, including 1 on the extra virtual desk from the "SingleWindow" case.
			{"WithOverview", 9, func() error { return nil }},
		}
		modeName = "ClamshellMode"
	}

	currentWindows := 0
	pv := perf.NewValues()
	for _, testCase := range testCases {
		s.Run(ctx, testCase.name, func(ctx context.Context, s *testing.State) {
			conns, err := ash.CreateWindows(ctx, tconn, cr, ui.PerftestURL, testCase.numWindows-currentWindows)
			if err != nil {
				s.Fatal("Failed to open windows: ", err)
			}
			conns.Close()
			currentWindows = testCase.numWindows

			// Enter overview, and then drag and snap a window to enter split view.
			if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
				s.Fatal("Failed to enter into the overview mode: ", err)
			}
			w, err := ash.FindFirstWindowInOverview(ctx, tconn)
			if err != nil {
				s.Fatal("Failed to find the window in the overview mode: ", err)
			}
			wCenterPoint := w.OverviewInfo.Bounds.CenterPoint()
			if err := pointerController.Press(ctx, wCenterPoint); err != nil {
				s.Fatal("Failed to start window drag from overview grid to snap: ", err)
			}
			// A window drag from a tablet overview grid must begin with a long press to
			// disambiguate from scrolling.
			if tabletMode {
				if err := testing.Sleep(ctx, time.Second); err != nil {
					s.Fatal("Failed to wait for touch to become long press, for window drag from overview grid to snap: ", err)
				}
			}
			if err := pointerController.Move(ctx, wCenterPoint, leftSnapPoint, time.Second); err != nil {
				s.Fatal("Failed during window drag from overview grid to snap: ", err)
			}
			if err := pointerController.Release(ctx); err != nil {
				s.Fatal("Failed to end window drag from overview grid to snap: ", err)
			}

			// id0 supposed to have the window id which is left-snapped.
			id0 := w.ID
			if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
				return w.ID == id0 && !w.IsAnimating && w.State == ash.WindowStateLeftSnapped
			}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
				s.Fatal("Failed to wait for the window to be left-snapped: ", err)
			}

			if err := testCase.customPrep(); err != nil {
				s.Fatal("Failed to prepare: ", err)
			}

			if err = cpu.WaitUntilIdle(ctx); err != nil {
				s.Fatal("Failed to wait: ", err)
			}

			hists, err := metrics.RunAndWaitAll(ctx, tconn, time.Second, func() (err error) {
				if err := pointerController.Press(ctx, dividerDragPointOne); err != nil {
					return errors.Wrap(err, "failed to start divider drag")
				}
				ended := false
				defer func() {
					if !ended {
						err = pointerController.Release(ctx)
					}
				}()
				if err := pointerController.Move(ctx, dividerDragPointOne, dividerDragPointTwo, 3*time.Second); err != nil {
					return errors.Wrap(err, "failed to drag divider slightly left")
				}
				if err := pointerController.Move(ctx, dividerDragPointTwo, dividerDragPointThree, 3*time.Second); err != nil {
					return errors.Wrap(err, "failed to drag divider all the way right")
				}
				if err := pointerController.Release(ctx); err != nil {
					return errors.Wrap(err, "failed to end divider drag")
				}
				ended = true
				return ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
					return w.ID == id0 && !w.IsAnimating && w.State == ash.WindowStateMaximized
				}, &testing.PollOptions{Timeout: 2 * time.Second})
			},
				fmt.Sprintf("Ash.SplitViewResize.PresentationTime.%s.%s", modeName, testCase.name),
				fmt.Sprintf("Ash.SplitViewResize.PresentationTime.MaxLatency.%s.%s", modeName, testCase.name))
			if err != nil {
				s.Fatal("Failed to drag or get the histogram: ", err)
			}

			for _, hist := range hists {
				latency, err := hist.Mean()
				if err != nil {
					s.Fatalf("Failed to get mean for histogram %s: %v", hist.Name, err)
				}
				pv.Set(perf.Metric{
					Name:      hist.Name,
					Unit:      "ms",
					Direction: perf.SmallerIsBetter,
				}, latency)
			}
		})
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
