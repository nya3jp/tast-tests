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

	if connected, err := display.PhysicalDisplayConnected(ctx, tconn); err != nil {
		s.Fatal("Failed to check if a physical display is connected or not: ", err)
	} else if !connected {
		s.Log("There are no physical displays and no data can be collected for this test")
		return
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		s.Fatal("Failed to ensure in clamshell mode: ", err)
	}
	defer cleanup(ctx)

	tew, err := input.Touchscreen(ctx)
	if err != nil {
		s.Fatal("Failed to access to the touch screen: ", err)
	}

	info, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain internal display info: ", err)
	}

	// Ensures in the landscape orientation; the following test scenario won't
	// succeed when the device is in the portrait mode.
	orientation, err := display.GetOrientation(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the orientation info: ", err)
	}
	rotation := -orientation.Angle
	if orientation.Type == display.OrientationPortraitPrimary {
		displayID := info.ID
		if err = display.SetDisplayRotationSync(ctx, tconn, displayID, display.Rotate90); err != nil {
			s.Fatal("Failed to rotate display: ", err)
		}
		if info, err = display.GetInternalInfo(ctx, tconn); err != nil {
			s.Fatal("Failed to refetch the display info: ", err)
		}
		defer display.SetDisplayRotationSync(ctx, tconn, info.ID, display.Rotate0)
		rotation += 90
	}
	if rotation != 0 {
		tew.SetRotation(rotation)
	}

	dsfX := float64(tew.Width()) / float64(info.Bounds.Width)
	dsfY := float64(tew.Height()) / float64(info.Bounds.Height)

	stw, err := tew.NewSingleTouchWriter()
	if err != nil {
		s.Fatal("Failed to create a single touch writer: ", err)
	}

	currentWindows := 0
	startX := tew.Width() / 2
	y := tew.Height() / 2
	midX := tew.Width() / 4
	endX := tew.Width() - 1

	pv := perf.NewValues()
	for _, c := range []struct {
		name       string
		numWindows int
		customPrep func() error
	}{
		{"SingleWindow", 1, func() error { return nil }},
		{"WithOverview", 8, func() error { return nil }},
		{"MultiWindow", 8, func() error {
			// Select a window from the right overview.
			var centerX, centerY input.TouchCoord
			w, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool {
				if w.OverviewInfo == nil {
					return false
				}
				bounds := w.OverviewInfo.Bounds
				centerX = input.TouchCoord(float64(bounds.Left+bounds.Width/2) * dsfX)
				centerY = input.TouchCoord(float64(bounds.Top+bounds.Height/2) * dsfY)
				return centerX >= 0 && centerX < tew.Width() && centerY >= 0 && centerY < tew.Height()
			})
			if err != nil {
				return errors.Wrap(err, "failed to find the window in the overview")
			}
			id1 := w.ID
			if err := stw.Move(centerX, centerY); err != nil {
				return errors.Wrapf(err, "failed to tap the center of %d", id1)
			}
			if err := stw.End(); err != nil {
				return errors.Wrapf(err, "failed to release the tap for %d", id1)
			}
			if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
				return w.ID == id1 && !w.IsAnimating && w.State == ash.WindowStateRightSnapped
			}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
				return errors.Wrapf(err, "failed to wait for window %d to be right-snapped: ", id1)
			}
			return nil
		}},
	} {
		s.Run(ctx, c.name, func(ctx context.Context, s *testing.State) {
			conns, err := ash.CreateWindows(ctx, cr, ui.PerftestURL, c.numWindows-currentWindows)
			if err != nil {
				s.Fatal("Failed to open windows: ", err)
			}
			conns.Close()
			currentWindows = c.numWindows

			// Drag the window to the side to achieve ths split-view state. Dragging
			// starts from the top-middle of the screen to detach the brwoser window
			// and ends at the middle-left of the screen to cause the left-snap.
			// Note that ash.SetWindowState(SnapLeft) is not available -- it's not
			// yet working well on the tablet mode.
			// TODO(mukai): add the fix on Chrome and use ash.SetWindowState here.
			if err := stw.Swipe(ctx, tew.Width()/2, 0, 0, tew.Height()/2, time.Second); err != nil {
				s.Fatal("Failed to swipe for snapping window: ", err)
			}
			if err := stw.End(); err != nil {
				s.Fatal("Failed to end the swipe: ", err)
			}

			// id0 supposed to have the window id which is left-snapped.
			var id0 int
			if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
				if !w.IsAnimating && w.State == ash.WindowStateLeftSnapped {
					id0 = w.ID
					return true
				}
				return false
			}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
				s.Fatal("Faild to wait for the window to be left-snapped: ", err)
			}

			if err := c.customPrep(); err != nil {
				s.Fatal("Failed to prepare: ", err)
			}

			if err = cpu.WaitUntilIdle(ctx); err != nil {
				s.Fatal("Failed to wait: ", err)
			}

			hists, err := metrics.Run(ctx, cr, func() (err error) {
				// The actual test scenario: firt swipe to left so that the window
				// shrinks slightly, and then swipe to the right-edge of the screen,
				// so the left-side window should be maximized again.
				if err := stw.Swipe(ctx, startX, y, midX, y, time.Second); err != nil {
					return errors.Wrap(err, "failed to swipe to the first point")
				}
				ended := false
				defer func() {
					if !ended {
						err = stw.End()
						if err == nil {
							err = ash.WaitWindowFinishAnimating(ctx, tconn, id0)
						}
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
			}, fmt.Sprintf("Ash.SplitViewResize.PresentationTime.TabletMode.%s", c.name))
			if err != nil {
				s.Fatal("Failed to drag or get the histogram: ", err)
			}

			latency, err := hists[0].Mean()
			if err != nil {
				s.Fatalf("Failed to get mean for histogram %s: %v", hists[0].Name, err)
			}
			pv.Set(perf.Metric{
				Name:      hists[0].Name,
				Unit:      "ms",
				Direction: perf.SmallerIsBetter,
			}, latency)
		})
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
