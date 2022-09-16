// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	uiperf "chromiumos/tast/local/bundles/cros/ui/perf"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/perfutil"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PerformantSplitViewPerf,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Measures smoothness of resizing split view windows with and without performant split view enabled",
		Contacts:     []string{"dandersson@chromium.org", "sammiequon@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Data:         []string{"heavy_resize.html"},
		Params: []testing.Param{{
			Name:    "flag_disabled",
			Val:     false,
			Timeout: 5 * time.Minute,
		}, {
			Val:     true,
			Timeout: 5 * time.Minute,
		}},
	})
}

func PerformantSplitViewPerf(ctx context.Context, s *testing.State) {
	// Reserve ten seconds for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	// Run an http server to serve the test contents for accessing from the chrome browsers.
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()
	url := server.URL + "/heavy_resize.html"

	var opt chrome.Option
	if s.Param().(bool) {
		opt = chrome.EnableFeatures("PerformantSplitViewResizing")
	} else {
		opt = chrome.DisableFeatures("PerformantSplitViewResizing")
	}

	cr, err := chrome.New(ctx, opt)
	if err != nil {
		s.Fatal("Failed to init: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		s.Fatal("Failed to ensure in tablet mode: ", err)
	}
	defer cleanup(cleanupCtx)

	// Ensures landscape orientation so this test can assume that windows snap on
	// the left and right.
	orientation, err := display.GetOrientation(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the orientation info: ", err)
	}
	if orientation.Type != display.OrientationLandscapePrimary {
		info, err := display.GetPrimaryInfo(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get the primary display info: ", err)
		}
		if err = display.SetDisplayRotationSync(ctx, tconn, info.ID, display.Rotate0); err != nil {
			s.Fatal("Failed to rotate display: ", err)
		}
	}

	pc, err := pointer.NewTouch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to set up the touch context: ", err)
	}
	defer pc.Close()

	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the primary display info: ", err)
	}

	yCenter := info.WorkArea.CenterY()
	// Compute the coordinates for the actual test scenario: drag the divider
	// slightly left and then all the way right. The left snapped window should
	// shrink and then expand and become maximized.
	dragPoints := []coords.Point{
		info.WorkArea.CenterPoint(),
		coords.NewPoint(info.WorkArea.Left+info.WorkArea.Width/4, yCenter),
		coords.NewPoint(info.WorkArea.Left+info.WorkArea.Width-1, yCenter),
		coords.NewPoint(info.WorkArea.CenterX()+info.WorkArea.Width/20, yCenter),
	}

	// Testing 2 patterns;
	// SingleWindow: there's a single window which is snapped to the left.
	// MultiWindow: one window is snapped to the left and another window
	//   is snapped to the right.
	type testCaseSlice []struct {
		name       string
		numWindows int
		customPrep func(ctx context.Context) error
	}

	testCases := testCaseSlice{
		{"SingleWindow", 1, func(ctx context.Context) error {
			return nil
		}},
		{"MultiWindow", 2, func(ctx context.Context) error {
			// Additional preparation for the multi-window; by default the right side
			// should be in the overview mode, so here selects one of the windows.
			// First, find the window which is in the overview and obtains the center
			// point of its bounds.
			w, err := ash.FindFirstWindowInOverview(ctx, tconn)
			if err != nil {
				return err
			}
			id1 := w.ID
			if err := pc.ClickAt(w.OverviewInfo.Bounds.CenterPoint())(ctx); err != nil {
				return errors.Wrapf(err, "failed to tap the center of %d", id1)
			}
			if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
				return w.ID == id1 && !w.IsAnimating && w.State == ash.WindowStateRightSnapped
			}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
				return errors.Wrap(err, "failed to wait for the condition")
			}
			return nil
		}},
	}

	defer ash.SetOverviewModeAndWait(cleanupCtx, tconn, false)
	currentWindows := 0
	runner := perfutil.NewRunner(cr.Browser())
	var id0 int
	for i, testCase := range testCases {
		s.Run(ctx, testCase.name, func(ctx context.Context, s *testing.State) {
			if err := ash.CreateWindows(ctx, tconn, cr, url, testCase.numWindows-currentWindows); err != nil {
				s.Fatal("Failed to open windows: ", err)
			}
			currentWindows = testCase.numWindows

			if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
				s.Fatal("Failed to enter into the overview mode: ", err)
			}

			// This is needed only once as the left-snapped window should stay as
			// left-snapped.
			if i == 0 {
				w, err := ash.FindFirstWindowInOverview(ctx, tconn)
				if err != nil {
					s.Fatal("Failed to find the window in the overview mode: ", err)
				}

				// id0 supposed to have the window id which is left-snapped.
				id0 = w.ID
				if err := ash.SetWindowStateAndWait(ctx, tconn, id0, ash.WindowStateLeftSnapped); err != nil {
					s.Fatal("Failed to snap window: ", err)
				}
			}

			if err := testCase.customPrep(ctx); err != nil {
				s.Fatal("Failed to prepare: ", err)
			}

			histogramNames := []string{
				fmt.Sprintf("Ash.SplitViewResize.PresentationTime.TabletMode.%s", testCase.name),
				fmt.Sprintf("Ash.SplitViewResize.PresentationTime.MaxLatency.TabletMode.%s", testCase.name),
				"Ash.SplitViewResize.AnimationSmoothness.DividerAnimation",
			}

			gestures := []uiauto.Action{
				pc.DragTo(dragPoints[1], time.Second),
				pc.DragTo(dragPoints[2], time.Second),
				pc.DragTo(dragPoints[3], time.Second),
			}
			// Note: in tablet mode, the split view divider will be still animating
			// at this point because ash.WaitForCondition does not check divider's
			// status. Still this is not a problem, as RunAndWaitAll function will
			// wait for the metrics for the divider animation which is generated
			// after the divider animation finishes.
			runner.RunMultiple(ctx, testCase.name, uiperf.Run(s,
				perfutil.RunAndWaitAll(tconn,
					uiauto.Combine("drag resizing the splitview",
						pc.Drag(dragPoints[0], gestures...),
						func(ctx context.Context) error {
							return ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
								return w.ID == id0 && !w.IsAnimating && w.State == ash.WindowStateLeftSnapped
							}, &testing.PollOptions{Timeout: 2 * time.Second})
						},
					),
					histogramNames...,
				)),
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
