// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/perfutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/metrics"
	chromeui "chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/pointer"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type splitViewResizeParam int

const (
	splitViewResizeClamshell splitViewResizeParam = iota
	splitViewResizeTablet
	splitViewResizeWebUI
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
				Name:    "clamshell_mode",
				Val:     splitViewResizeClamshell,
				Fixture: "chromeLoggedIn",
			},
			{
				ExtraSoftwareDeps: []string{"tablet_mode"},
				Val:               splitViewResizeTablet,
				Fixture:           "chromeLoggedIn",
			},
			{
				Name:              "webui",
				ExtraSoftwareDeps: []string{"tablet_mode"},
				Val:               splitViewResizeWebUI,
			},
		},
	})
}

func SplitViewResizePerf(ctx context.Context, s *testing.State) {
	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	param := s.Param().(splitViewResizeParam)
	tabletMode := param != splitViewResizeClamshell
	webUITabStrip := param == splitViewResizeWebUI

	var cr *chrome.Chrome
	if webUITabStrip {
		var err error
		// TODO(mukai): remove this when WebUITabStrip is enabled by default.
		if cr, err = chrome.New(ctx, chrome.EnableFeatures("WebUITabStrip")); err != nil {
			s.Fatal("Failed to init: ", err)
		}
		defer cr.Close(ctx)
	} else {
		cr = s.FixtValue().(*chrome.Chrome)
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
	if orientation.Type == display.OrientationPortraitPrimary {
		info, err := display.GetPrimaryInfo(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get the primary display info: ", err)
		}
		if err = display.SetDisplayRotationSync(ctx, tconn, info.ID, display.Rotate90); err != nil {
			s.Fatal("Failed to rotate display: ", err)
		}
		defer display.SetDisplayRotationSync(ctx, tconn, info.ID, display.Rotate0)
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
		info.WorkArea.CenterPoint(),
	}
	if tabletMode {
		// In tablet mode, the last point should be moved slightly right, so that
		// the split-view controller moves the divider slightly.
		dragPoints[3].X += info.WorkArea.Width / 20
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
		customPrep func(ctx context.Context) error
	}
	var testCases testCaseSlice
	var modeName string
	if tabletMode {
		testCases = testCaseSlice{
			{"SingleWindow", 1, func(ctx context.Context) error {
				if !webUITabStrip {
					return nil
				}
				// Click the toggle button to open the WebUI tabstrip.
				params := chromeui.FindParams{
					Role:       chromeui.RoleTypeButton,
					Attributes: map[string]interface{}{"name": regexp.MustCompile("toggle tab strip")},
				}
				toggleButton, err := chromeui.Find(ctx, tconn, params)
				if err != nil {
					return err
				}
				waitOptions := &testing.PollOptions{Timeout: 10 * time.Second, Interval: time.Second}
				if err := toggleButton.WaitLocationStable(ctx, waitOptions); err != nil {
					return err
				}
				if err := pointer.Click(ctx, pointerController, toggleButton.Location.CenterPoint()); err != nil {
					return err
				}
				if err := toggleButton.WaitLocationStable(ctx, waitOptions); err != nil {
					return err
				}
				// Disable automation features explicitly, so that further operations
				// won't be affected by accessibility events. See
				// https://crbug.com/1096719 and https://crbug.com/1111137.
				return tconn.ResetAutomation(ctx)
			}},
			{"WithOverview", 8, func(ctx context.Context) error {
				// No need to control the WebUI tabstrip, as it remains open on the
				// browser window of the left side.
				return nil
			}},
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
				if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
					return w.ID == id1 && !w.IsAnimating && w.State == ash.WindowStateRightSnapped
				}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
					return errors.Wrap(err, "failed to wait for the condition")
				}

				if !webUITabStrip {
					return nil
				}

				// Open the WebUI tabstrip of the browser window of the right side.
				params := chromeui.FindParams{
					Role:       chromeui.RoleTypeButton,
					Attributes: map[string]interface{}{"name": regexp.MustCompile("toggle tab strip")},
				}
				toggleButtons, err := chromeui.FindAll(ctx, tconn, params)
				if err != nil {
					return err
				}
				for _, toggleButton := range toggleButtons {
					waitOptions := &testing.PollOptions{Timeout: 10 * time.Second, Interval: time.Second}
					if err := toggleButton.WaitLocationStable(ctx, waitOptions); err != nil {
						return err
					}
					// The toggle button on the lefthand-side browser window should be
					// skipped.
					if toggleButton.Location.CenterPoint().X < info.Bounds.CenterPoint().X {
						continue
					}
					if err := pointer.Click(ctx, pointerController, toggleButton.Location.CenterPoint()); err != nil {
						return err
					}
					if err := toggleButton.WaitLocationStable(ctx, waitOptions); err != nil {
						return err
					}
					break
				}
				// Disable automation features explicitly, so that further operations
				// won't be affected by accessibility events. See
				// https://crbug.com/1096719 and https://crbug.com/1111137.
				return tconn.ResetAutomation(ctx)
			}},
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
				return tconn.ResetAutomation(ctx)
			}},
			// 9 windows, including 1 on the extra virtual desk from the "SingleWindow" case.
			{"WithOverview", 9, func(context.Context) error { return nil }},
		}
		modeName = "ClamshellMode"
	}

	currentWindows := 0
	runner := perfutil.NewRunner(cr)
	var id0 int
	for i, testCase := range testCases {
		s.Run(ctx, testCase.name, func(ctx context.Context, s *testing.State) {
			conns, err := ash.CreateWindows(ctx, tconn, cr, ui.PerftestURL, testCase.numWindows-currentWindows)
			if err != nil {
				s.Fatal("Failed to open windows: ", err)
			}
			if err := conns.Close(); err != nil {
				s.Fatal("Failed to close the connections to the windows: ", err)
			}
			currentWindows = testCase.numWindows

			// Enter overview, and then drag and snap a window to enter split view.
			if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
				s.Fatal("Failed to enter into the overview mode: ", err)
			}

			// In tablet-mode, this is needed only once as the left-snapped window
			// should stay as left-snapped.
			if !tabletMode || i == 0 {
				w, err := ash.FindFirstWindowInOverview(ctx, tconn)
				if err != nil {
					s.Fatal("Failed to find the window in the overview mode: ", err)
				}
				wCenterPoint := w.OverviewInfo.Bounds.CenterPoint()

				if err := pointerController.Press(ctx, wCenterPoint); err != nil {
					s.Fatal("Failed to start window drag from overview grid to snap: ", err)
				}
				ended := false
				defer func() {
					if !ended {
						if err := pointerController.Release(ctx); err != nil {
							s.Error("Failed to release the pointer: ", err)
						}
					}
				}()
				// A window drag from a tablet overview grid must begin with a long press to
				// disambiguate from scrolling.
				if tabletMode {
					if err := testing.Sleep(ctx, time.Second); err != nil {
						s.Fatal("Failed to wait for touch to become long press, for window drag from overview grid to snap: ", err)
					}
				}
				if err := pointerController.Move(ctx, wCenterPoint, leftSnapPoint, 200*time.Millisecond); err != nil {
					s.Fatal("Failed during window drag from overview grid to snap: ", err)
				}
				if err := pointerController.Release(ctx); err != nil {
					s.Fatal("Failed to end window drag from overview grid to snap: ", err)
				}
				ended = true

				// id0 supposed to have the window id which is left-snapped.
				id0 = w.ID
			}

			if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
				return w.ID == id0 && !w.IsAnimating && w.State == ash.WindowStateLeftSnapped
			}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
				s.Fatal("Failed to wait for the window to be left-snapped: ", err)
			}

			if err := testCase.customPrep(ctx); err != nil {
				s.Fatal("Failed to prepare: ", err)
			}

			const dividerSmoothnessName = "Ash.SplitViewResize.AnimationSmoothness.DividerAnimation"
			histogramNames := []string{
				fmt.Sprintf("Ash.SplitViewResize.PresentationTime.%s.%s", modeName, testCase.name),
				fmt.Sprintf("Ash.SplitViewResize.PresentationTime.MaxLatency.%s.%s", modeName, testCase.name),
			}

			// In tablet mode, there is a divider which snaps to the closest fixed ratio on release. The written histogram is a smoothness histogram.
			if modeName == "TabletMode" {
				histogramNames = append(histogramNames, dividerSmoothnessName)
			}
			if tabletMode && testCase.name == "WithOverview" {
				histogramNames = append(histogramNames,
					"Ash.Overview.AnimationSmoothness.Enter.SplitView",
					"Ash.Overview.AnimationSmoothness.Exit.SplitView",
				)
			}
			runner.RunMultiple(ctx, s, testCase.name, perfutil.RunAndWaitAll(tconn, func(ctx context.Context) (err error) {
				if tabletMode && testCase.name == "WithOverview" {
					if err := ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
						return errors.Wrap(err, "failed to exit overview at right")
					}
					if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
						return errors.Wrap(err, "failed to start overview at right")
					}
				}
				if err := pointerController.Press(ctx, dragPoints[0]); err != nil {
					return errors.Wrap(err, "failed to start divider drag")
				}
				ended := false
				defer func() {
					if !ended {
						if releaseErr := pointerController.Release(ctx); releaseErr != nil {
							err = releaseErr
						}
					}
				}()
				if err := pointerController.Move(ctx, dragPoints[0], dragPoints[1], time.Second); err != nil {
					return errors.Wrap(err, "failed to drag divider slightly left")
				}
				if err := pointerController.Move(ctx, dragPoints[1], dragPoints[2], time.Second); err != nil {
					return errors.Wrap(err, "failed to drag divider all the way right")
				}
				if err := pointerController.Move(ctx, dragPoints[2], dragPoints[3], time.Second); err != nil {
					return errors.Wrap(err, "failed to drag divider back to middle position")
				}
				if !tabletMode {
					if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
						return w.ID == id0 && w.BoundsInRoot.Right() == dragPoints[3].X
					}, &testing.PollOptions{Timeout: 2 * time.Second}); err != nil {
						return errors.Wrap(err, "failed to wait for resize to finish")
					}
				}
				if err := pointerController.Release(ctx); err != nil {
					return errors.Wrap(err, "failed to end divider drag")
				}
				ended = true
				if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
					return w.ID == id0 && !w.IsAnimating && w.State == ash.WindowStateLeftSnapped
				}, &testing.PollOptions{Timeout: 2 * time.Second}); err != nil {
					return errors.Wrap(err, "failed to wait for the window state back to original position")
				}
				// Note: in tablet mode, the split view divider will be still animating
				// at this point because ash.WaitForCondition does not check divider's
				// status. Still this is not a problem, as RunAndWaitAll function will
				// wait for the metrics for the divider animation which is generated
				// after the divider animation finishes.
				return nil
			}, histogramNames...),
				func(ctx context.Context, pv *perfutil.Values, hists []*metrics.Histogram) error {
					for _, hist := range hists {
						value, err := hist.Mean()
						if err != nil {
							return errors.Wrapf(err, "failed to get mean for histogram %s", hist.Name)
						}
						testing.ContextLog(ctx, hist.Name, " = ", value)
						unit := "ms"
						direction := perf.SmallerIsBetter
						if strings.Contains(hist.Name, "AnimationSmoothnes") {
							unit = "percent"
							direction = perf.BiggerIsBetter
						}
						pv.Append(perf.Metric{
							Name:      hist.Name,
							Unit:      unit,
							Direction: direction,
						}, value)
					}
					return nil
				})
		})
	}

	if err := runner.Values().Save(ctx, s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
