// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SplitViewResizePerf,
		Desc:         "Measures smoothness of resizing windows in the split view of the tablet mode",
		Contacts:     []string{"mukai@chromium.org", "sammiequon@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "tablet_mode"},
		Pre:          chrome.LoggedIn(),
		Timeout:      5 * time.Minute,
	})
}

func SplitViewResizePerf(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	// This test cannot collect data points without a physical display, and
	// there's no ways to exclude this test. See https://crbug.com/1049430.
	if connected, err := display.PhysicalDisplayConnected(ctx, tconn); err != nil {
		s.Fatal("Failed to check if a physical display is connected or not: ", err)
	} else if !connected {
		s.Log("There are no physical displays and no data can be collected for this test")
		return
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		s.Fatal("Failed to ensure in tablet mode: ", err)
	}
	defer cleanup(ctx)

	tew, err := input.Touchscreen(ctx)
	if err != nil {
		s.Fatal("Failed to access to the touch screen: ", err)
	}
	defer tew.Close()

	// Ensures in the landscape orientation; the following test scenario won't
	// succeed when the device is in the portrait mode.
	orientation, err := display.GetOrientation(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the orientation info: ", err)
	}
	rotation := -orientation.Angle
	if orientation.Type == display.OrientationPortraitPrimary {
		info, err := display.GetInternalInfo(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to obtain internal display info: ", err)
		}
		if err = display.SetDisplayRotationSync(ctx, tconn, info.ID, display.Rotate90); err != nil {
			s.Fatal("Failed to rotate display: ", err)
		}
		defer display.SetDisplayRotationSync(ctx, tconn, info.ID, display.Rotate0)
		rotation += 90
	}
	tew.SetRotation(rotation)

	tcc, err := ash.NewTouchCoordConverter(ctx, tconn, tew)
	if err != nil {
		s.Fatal("Failed to create touch coord converter: ", err)
	}

	stw, err := tew.NewSingleTouchWriter()
	if err != nil {
		s.Fatal("Failed to create a single touch writer: ", err)
	}

	// Computing the coordinates for the gesture of resizing, see also the
	// comments around metrics.Run.
	// y: shares the same value (at the middle of the screen) to move horizontaly.
	// splitX: in the middle of the screen (should have the splitter).
	// midX: in the center of the left-snapped window.
	// endX: the right edge of the screen. Note that tew.Width() is outside of the
	//   visible area, thus subtracted by 1.
	splitX := tew.Width() / 2
	y := tew.Height() / 2
	midX := tew.Width() / 4
	endX := tew.Width() - 1

	currentWindows := 0
	pv := perf.NewValues()

	// Testing 3 patterns;
	// SingleWindow: there's a single window which is snapped to the left.
	// WithOverview: one window is snapped to the left, and the right area is
	//   in the overview mode.
	// MultiWindow: one window is snapped to the left and another window is
	//   snapped to the right.
	for _, testCase := range []struct {
		name       string
		numWindows int
		customPrep func() error
	}{
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
			centerX, centerY := tcc.ConvertLocation(w.OverviewInfo.Bounds.CenterPoint())
			// Tap the center of the overview window, and wait it to be snapped to
			// the right.
			if err := stw.Move(centerX, centerY); err != nil {
				return errors.Wrapf(err, "failed to tap the center of %d", id1)
			}
			if err := stw.End(); err != nil {
				return errors.Wrapf(err, "failed to release the tap for %d", id1)
			}
			return ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
				return w.ID == id1 && !w.IsAnimating && w.State == ash.WindowStateRightSnapped
			}, &testing.PollOptions{Timeout: 5 * time.Second})
		}},
	} {
		s.Run(ctx, testCase.name, func(ctx context.Context, s *testing.State) {
			conns, err := ash.CreateWindows(ctx, cr, ui.PerftestURL, testCase.numWindows-currentWindows)
			if err != nil {
				s.Fatal("Failed to open windows: ", err)
			}
			conns.Close()
			currentWindows = testCase.numWindows

			// Entering into the overview mode and then drag the window to the side to
			// achieve ths split-view state. The operation is
			// * long press at the center of the target window to start dragging
			// * move the touch point to the middle-left of the screen to snap left
			// Note that ash.SetWindowState(SnapLeft) is not available -- it's not yet
			// working well on the tablet mode. See also: https://crbug.com/1045990.
			// TODO(mukai): use ash.SetWindowState here once it becomes available.
			if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
				s.Fatal("Failed to enter into the overview mode: ", err)
			}
			w, err := ash.FindFirstWindowInOverview(ctx, tconn)
			if err != nil {
				s.Fatal("Failed to find the window in the overview mode: ", err)
			}
			centerX, centerY := tcc.ConvertLocation(w.OverviewInfo.Bounds.CenterPoint())
			if err := stw.LongPressAt(ctx, centerX, centerY); err != nil {
				s.Fatal("Failed to long-press to select the target window: ", err)
			}
			if err := stw.Swipe(ctx, centerX, centerY, 0, tew.Height()/2, time.Second); err != nil {
				s.Fatal("Failed to swipe for snapping window: ", err)
			}
			if err := stw.End(); err != nil {
				s.Fatal("Failed to end the swipe: ", err)
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

			hists, err := metrics.Run(ctx, tconn, func() (err error) {
				// The actual test scenario: first swipe to left so that the window
				// shrinks slightly, and then swipe to the right-edge of the screen,
				// so the left-side window should be maximized again.
				if err := stw.Swipe(ctx, splitX, y, midX, y, time.Second); err != nil {
					return errors.Wrap(err, "failed to swipe to the first point")
				}
				ended := false
				defer func() {
					if !ended {
						err = stw.End()
					}
				}()
				if err := stw.Swipe(ctx, midX, y, endX, y, 3*time.Second); err != nil {
					return errors.Wrap(err, "failed to swipe to the end")
				}
				if err := stw.End(); err != nil {
					return errors.Wrap(err, "failed to end the swipe")
				}
				ended = true
				return ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
					return w.ID == id0 && !w.IsAnimating && w.State == ash.WindowStateMaximized
				}, &testing.PollOptions{Timeout: 2 * time.Second})
			},
				fmt.Sprintf("Ash.SplitViewResize.PresentationTime.TabletMode.%s", testCase.name),
				fmt.Sprintf("Ash.SplitViewResize.PresentationTime.MaxLatency.TabletMode.%s", testCase.name))
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
