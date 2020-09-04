// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/perfutil"
	"chromiumos/tast/local/bundles/cros/ui/pointer"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	chromeui "chromiumos/tast/local/chrome/ui"
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
		Pre:          chrome.LoggedIn(),
		Params: []testing.Param{
			{
				Name: "clamshell_mode",
				Val:  false,
			},
			{
				ExtraSoftwareDeps: []string{"tablet_mode"},
				Val:               true,
			},
		},
	})
}

func SplitViewResizePerf(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	tabletMode := s.Param().(bool)
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
	yCenter := info.WorkArea.CenterY()
	// Compute the coordinates where we will drag an overview window to snap left.
	leftSnapPoint := coords.NewPoint(0, yCenter)
	// Compute the coordinates for the actual test scenario: drag the divider
	// slightly left and then all the way right. The left snapped window should
	// shrink and then expand and become maximized.
	dragPoints := []coords.Point{
		info.WorkArea.CenterPoint(),
		coords.NewPoint(info.WorkArea.Left+info.WorkArea.Width/4, yCenter),
		coords.NewPoint(info.WorkArea.Left+info.WorkArea.Width-1, yCenter),
	}
	if tabletMode {
		// In tablet mode, the last point should be moved slightly left, so that
		// the split-view controller moves the divider slightly.
		dragPoints[2].X -= info.WorkArea.Width / 20
	}

	// Testing 3 patterns;
	// SingleWindow: there's a single window which is snapped to the left.
	// WithOverview: one window is snapped to the left, and the right area is
	//   in the overview mode.
	// MultiWindow (tablet only): one window is snapped to the left and another
	//   window is snapped to the right.
	type testCaseSlice []struct {
		name       string
		numWindows int

		// customPrep is a function to be called before the dragging action starts.
		customPrep func(ctx context.Context) error

		// customCleanup is a function to be called after the test scenario ends.
		customCleanup func(ctx context.Context) error
	}
	var testCases testCaseSlice
	var modeName string
	if tabletMode {
		testCases = testCaseSlice{
			{"SingleWindow", 1, nil, nil},
			{"WithOverview", 8, nil, nil},
			{"MultiWindow", 8, func(ctx context.Context) error {
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
			}, nil},
		}
		modeName = "TabletMode"
	} else {
		testCases = testCaseSlice{
			{"SingleWindow", 2, func(ctx context.Context) error {
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
				// Disable automation features explicitly, so that further operations
				// won't be affected by accessibility events. See
				// https://crbug.com/1096719 and https://crbug.com/1111137.
				if err := tconn.Eval(ctx, "tast.promisify(chrome.autotestPrivate.disableAutomation)()", nil); err != nil {
					return errors.Wrap(err, "failed to disable the automation feature")
				}
				// Reload the test conn explicitly to clear the cache of accessibility
				// tree.
				if err := tconn.Eval(ctx, "location.reload()", nil); err != nil {
					return errors.Wrap(err, "failed to reload the test extension")
				}
				if err := tconn.WaitForExpr(ctx, "document.readyState === 'complete'"); err != nil {
					return errors.Wrap(err, "failed to wait for the ready state")
				}
				return nil
			}, func(ctx context.Context) error {
				// At the end of each run, the desk created in the customPrep above
				// should be removed to return back to the original status. Note that
				// keeping the other desk will not satisfy the testing condition. If
				// there's a single window in the current desk, it isn't entering into
				// clamshell splitview mode.
				return ash.RemoveActiveDesk(ctx, tconn)
			}},
			{"WithOverview", 8, nil, nil},
		}
		modeName = "ClamshellMode"
	}

	currentWindows := 0
	runner := perfutil.NewRunner(cr)
	for _, testCase := range testCases {
		s.Run(ctx, testCase.name, func(ctx context.Context, s *testing.State) {
			conns, err := ash.CreateWindows(ctx, tconn, cr, ui.PerftestURL, testCase.numWindows-currentWindows)
			if err != nil {
				s.Fatal("Failed to open windows: ", err)
			}
			if err := conns.Close(); err != nil {
				s.Fatal("Failed to close the connections to the windows: ", err)
			}
			currentWindows = testCase.numWindows

			histogramNames := []string{
				fmt.Sprintf("Ash.SplitViewResize.PresentationTime.%s.%s", modeName, testCase.name),
				fmt.Sprintf("Ash.SplitViewResize.PresentationTime.MaxLatency.%s.%s", modeName, testCase.name),
			}
			// In tablet mode, there is a divider which snaps to the closest fixed
			// ratio on release. The written histogram is a smoothness histogram.
			if modeName == "TabletMode" {
				histogramNames = append(histogramNames, "Ash.SplitViewResize.AnimationSmoothness.DividerAnimation")
			}
			// In tablet mode with overview status, the overview animation smoothness
			// should also be measured.
			if tabletMode && currentWindows > 1 {
				histogramNames = append(histogramNames,
					"Ash.Overview.AnimationSmoothness.Enter.SplitView",
					"Ash.Overview.AnimationSmoothness.Exit.SplitView",
				)
			}

			if err = cpu.WaitUntilIdle(ctx); err != nil {
				s.Fatal("Failed to wait: ", err)
			}

			// The main test scenario.
			// 1. enter into the overview mode, drag a window to the left-snapped.
			// 2. if the righthand side is in overview mode, exit the overview mode
			//    and re-enter.
			// 3. custom preparation.
			// 4. start the resize session; moves slightly to the left, and then moves
			//    to the right edge.
			runner.RunMultiple(ctx, s, testCase.name, perfutil.RunAndWaitAll(tconn, func(ctx context.Context) (err error) {
				// 1. enter overview, and then drag and snap a window to enter split view.
				if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
					return errors.Wrap(err, "failed to enter into the overview mode")
				}
				w, findErr := ash.FindFirstWindowInOverview(ctx, tconn)
				if findErr != nil {
					return errors.Wrap(err, "failed to find the window in the overview mode")
				}
				wCenterPoint := w.OverviewInfo.Bounds.CenterPoint()
				ended := false
				if err := pointerController.Press(ctx, wCenterPoint); err != nil {
					return errors.Wrap(err, "failed to start window drag from overview grid to snap")
				}
				defer func() {
					if !ended {
						if releaseErr := pointerController.Release(ctx); releaseErr != nil {
							testing.ContextLog(ctx, "Failed to release the pointer: ", releaseErr)
							if err == nil {
								err = releaseErr
							}
						}
					}
				}()

				// A window drag from a tablet overview grid must begin with a long press to
				// disambiguate from scrolling.
				if tabletMode {
					if err := testing.Sleep(ctx, time.Second); err != nil {
						return errors.Wrap(err, "failed to wait for touch to become long press, for window drag from overview grid to snap")
					}
				}
				if err := pointerController.Move(ctx, wCenterPoint, leftSnapPoint, 200*time.Millisecond); err != nil {
					return errors.Wrap(err, "failed during window drag from overview grid to snap")
				}
				if err := pointerController.Release(ctx); err != nil {
					return errors.Wrap(err, "failed to end window drag from overview grid to snap")
				}
				ended = true

				// id0 supposed to have the window id which is left-snapped.
				id0 := w.ID
				if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
					return w.ID == id0 && !w.IsAnimating && w.State == ash.WindowStateLeftSnapped
				}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
					return errors.Wrap(err, "failed to wait for the window to be left-snapped")
				}

				// 2. close the right-overview mode and re-enter into right overview
				// mode this emits Ash.Overview.AnimationSmoothness.{Enter,Exit}.SplitView.
				// See: https://crbug.com/1124081.
				if tabletMode && currentWindows > 1 {
					if err := ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
						return errors.Wrap(err, "failed to exit from the right overview mode")
					}
					if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
						return errors.Wrap(err, "failed to enter into the overview mode at right")
					}
				}

				// 3. custom preparation.
				if testCase.customPrep != nil {
					if err := testCase.customPrep(ctx); err != nil {
						return errors.Wrap(err, "failed to prepare")
					}
				}
				if testCase.customCleanup != nil {
					defer testCase.customCleanup(ctx)
				}

				// 4. generate dragging events.
				if !tabletMode {
					// In clamshell mode, the window width does not stick to the half of
					// the screen exactly, and the previous drag will end up with a
					// slightly different width. So checking the starting position again.
					w, err := ash.FindWindow(ctx, tconn, func(window *ash.Window) bool {
						return id0 == w.ID
					})
					if err != nil {
						return errors.Wrap(err, "failed to find the window")
					}
					dragPoints[0].X = w.BoundsInRoot.Right()
				}
				if err := pointerController.Press(ctx, dragPoints[0]); err != nil {
					return errors.Wrap(err, "failed to start divider drag")
				}
				ended = false
				for i := 0; i < len(dragPoints)-1; i++ {
					if err := pointerController.Move(ctx, dragPoints[i], dragPoints[i+1], 500*time.Millisecond); err != nil {
						return errors.Wrapf(err, "failed to drag divider from %v to %v at step %d", dragPoints[i], dragPoints[i+1], i)
					}
				}
				if err := pointerController.Release(ctx); err != nil {
					return errors.Wrap(err, "failed to end divider drag")
				}
				ended = true
				if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
					return w.ID == id0 && !w.IsAnimating && w.State == ash.WindowStateMaximized
				}, &testing.PollOptions{Timeout: 2 * time.Second}); err != nil {
					return errors.Wrap(err, "failed to wait for the window state back to original position")
				}
				return nil
			}, histogramNames...),
				perfutil.StoreWithHeuristics)
		})
	}

	if err := runner.Values().Save(ctx, s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
