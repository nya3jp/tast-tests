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
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
)

type overflowShelfTest struct {
	enabled bool // Whether overflow shelf should be tested.
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         HotseatAnimation,
		Desc:         "Measures the framerate of the hotseat animation in tablet mode",
		Contacts:     []string{"newcomer@chromium.org", "manucornet@chromium.org", "andrewxu@chromium.org", "cros-shelf-prod-notifications@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "tablet_mode"},
		Pre:          ash.LoggedInWith100DummyApps(),
		Timeout:      8 * time.Minute,
		Params: []testing.Param{{
			Name: "non_overflow_shelf",
			Val: overflowShelfTest{
				enabled: false,
			},
		}, {
			Name: "overflow_shelf",
			Val: overflowShelfTest{
				enabled: true,
			},
		}},
	})
}

// HotseatAnimation measures the performance of hotseat background bounds animation.
func HotseatAnimation(ctx context.Context, s *testing.State) {
	// TODO(newcomer): please record performance of navigation widget (https://crbug.com/1065405).
	const (
		extendedHotseatHistogram = "Ash.HotseatTransition.AnimationSmoothness.TransitionToExtendedHotseat"
		hiddenHotseatHistogram   = "Ash.HotseatTransition.AnimationSmoothness.TransitionToHiddenHotseat"
		shownHotseatHistogram    = "Ash.HotseatTransition.AnimationSmoothness.TransitionToShownHotseat"
	)

	cr := s.PreValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	orientation, err := display.GetOrientation(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the display rotation: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		s.Fatal("Failed to ensure in tablet mode: ", err)
	}
	defer cleanup(ctx)

	// Prepare the touch screen as this test requires touch scroll events.
	tsw, err := input.Touchscreen(ctx)
	if err != nil {
		s.Fatal("Failed to create touch screen event writer: ", err)
	}
	defer tsw.Close()
	if err := tsw.SetRotation(-orientation.Angle); err != nil {
		s.Fatal("Failed to set rotation: ", err)
	}

	stw, err := tsw.NewSingleTouchWriter()
	if err != nil {
		s.Fatal("Failed to create single touch writer: ", err)
	}
	defer stw.Close()

	// At login, we should only have Chrome in the Shelf.
	if shelfItems, err := ash.ShelfItems(ctx, tconn); err != nil {
		s.Fatal("Failed to get shelf items: ", err)
	} else if len(shelfItems) != 1 {
		s.Fatalf("Unexpected num of apps in the shelf: got %d; want 1", len(shelfItems))
	}

	if s.Param().(overflowShelfTest).enabled {
		if err := ash.EnterShelfOverflow(ctx, tconn); err != nil {
			s.Fatal(err, "Failed to enter overflow shelf")
		}
	}

	// Wait for the animations to complete and for things to settle down.
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed waiting for CPU to become idle: ", err)
	}

	pv := perf.NewValues()

	// Collect metrics data from hiding hotseat by window creation.
	histogramGroup, err := metrics.Run(ctx, tconn, func() error {
		const numWindows = 1
		conns, err := ash.CreateWindows(ctx, cr, ui.EmptyURL, numWindows)
		if err != nil {
			return errors.Wrap(err, "failed to open browser windows: ")
		}
		if err := conns.Close(); err != nil {
			return errors.Wrap(err, "failed to close the connection to a browser window")
		}

		if err := ash.WaitForHotseatAnimatingToIdealState(ctx, tconn, ash.ShelfHidden); err != nil {
			return err
		}

		return nil
	}, hiddenHotseatHistogram)
	if err != nil {
		s.Fatal("Failed to get mean histograms from hiding hotseat by window creation: ", err)
	}

	// Save metrics data from hiding hotseat by window creation.
	for _, h := range histogramGroup {
		mean, err := h.Mean()
		if err != nil {
			s.Fatalf("Failed to get mean for histogram %s: %v", h.Name, err)
		}

		pv.Set(perf.Metric{
			Name:      h.Name + ".WindowCreation",
			Unit:      "percent",
			Direction: perf.BiggerIsBetter,
		}, mean)
	}

	// Collect metrics data from entering/exiting overview.
	histogramGroup, err = metrics.Run(ctx, tconn, func() error {
		addNewTab := func(ctx context.Context, cr *chrome.Chrome, url string) error {
			conn, err := cr.NewConn(ctx, url)
			if err != nil {
				return errors.Wrap(err, "cannot create a new tab")
			}

			conn.Close()
			return nil
		}
		if err := addNewTab(ctx, cr, ui.PerftestURL); err != nil {
			return err
		}

		if err := ash.DragToShowOverview(ctx, tsw.Width(), tsw.Height(), stw, tconn); err != nil {
			return errors.Wrap(err, "failed to drag from bottom of the screen to show overview")
		}

		enterHomeLauncherFromOverview := func(ctx context.Context, tconn *chrome.TestConn, stw *input.SingleTouchEventWriter, tsw *input.TouchscreenEventWriter) error {
			const errorMsg = "failed to enter home launcher from overview"

			pressX := tsw.Width() * 5 / 6
			pressY := tsw.Height() / 2
			if err := stw.Swipe(ctx, pressX, pressY, pressX+5, pressY-5, 200*time.Millisecond); err != nil {
				return errors.Wrap(err, errorMsg)
			}
			if err := stw.End(); err != nil {
				return errors.Wrap(err, errorMsg)
			}
			if err := ash.WaitForOverviewState(ctx, tconn, ash.Hidden); err != nil {
				return errors.Wrap(err, errorMsg)
			}
			if err := ash.WaitForHotseatAnimatingToIdealState(ctx, tconn, ash.ShelfShownHomeLauncher); err != nil {
				return errors.Wrap(err, errorMsg)
			}

			return nil
		}
		if err := enterHomeLauncherFromOverview(ctx, tconn, stw, tsw); err != nil {
			return err
		}

		if err := ash.DragToShowOverview(ctx, tsw.Width(), tsw.Height(), stw, tconn); err != nil {
			return errors.Wrap(err, "failed to drag from bottom of the screen to show overview")
		}

		enterInAppModeFromOverview := func(ctx context.Context, tconn *chrome.TestConn, stw *input.SingleTouchEventWriter, tsw *input.TouchscreenEventWriter) error {
			pressX := tsw.Width() / 3
			pressY := tsw.Height() / 3

			if err := stw.Swipe(ctx, pressX, pressY, pressX+5, pressY-5, 200*time.Millisecond); err != nil {
				return err
			}
			if err := stw.End(); err != nil {
				return err
			}

			if err := ash.WaitForOverviewState(ctx, tconn, ash.Hidden); err != nil {
				return err
			}

			if err := ash.WaitForHotseatAnimatingToIdealState(ctx, tconn, ash.ShelfHidden); err != nil {
				return err
			}

			return nil
		}
		if err := enterInAppModeFromOverview(ctx, tconn, stw, tsw); err != nil {
			return err
		}

		enterHomeLauncherFromInAppMode := func(ctx context.Context, tconn *chrome.TestConn, stw *input.SingleTouchEventWriter, tsw *input.TouchscreenEventWriter) error {
			startX := tsw.Width() / 2
			startY := tsw.Height() - 1

			endX := startX
			endY := tsw.Height() / 2

			if err := stw.Swipe(ctx, startX, startY, endX, endY, 200*time.Millisecond); err != nil {
				return err
			}

			if err := stw.End(); err != nil {
				return err
			}

			if err := ash.WaitForLauncherState(ctx, tconn, ash.FullscreenAllApps); err != nil {
				return err
			}

			if err := ash.WaitForHotseatAnimatingToIdealState(ctx, tconn, ash.ShelfShownHomeLauncher); err != nil {
				return err
			}

			return nil
		}
		if err := enterHomeLauncherFromInAppMode(ctx, tconn, stw, tsw); err != nil {
			return err
		}

		return nil
	},
		shownHotseatHistogram,
		extendedHotseatHistogram)
	if err != nil {
		s.Fatal("Failed to get mean histogram from entering/exiting overview: ", err)
	}

	// Save metrics data from entering/exiting overview.
	for _, h := range histogramGroup {
		mean, err := h.Mean()
		if err != nil {
			s.Fatalf("Failed to get mean for histogram %s: %v", h.Name, err)
		}

		pv.Set(perf.Metric{
			Name:      h.Name,
			Unit:      "percent",
			Direction: perf.BiggerIsBetter,
		}, mean)
	}

	// Collect metrics data from hiding hotseat by window activation.
	histogramGroup, err = metrics.Run(ctx, tconn, func() error {
		hideHotseatByActivatingWindow := func(ctx context.Context, tconn *chrome.TestConn, tsw *input.TouchscreenEventWriter, stw *input.SingleTouchEventWriter) error {
			// Verify the initial hotseat state before hiding.
			if err := ash.WaitForHotseatAnimatingToIdealState(ctx, tconn, ash.ShelfShownHomeLauncher); err != nil {
				return err
			}

			scrollableShelfInfo, err := ash.FetchScrollableShelfInfo(ctx, tconn)
			if err != nil {
				return err
			}

			if len(scrollableShelfInfo.IconsBoundsInScreen) == 0 {
				return errors.New("failed to activate a window: got 0 shelf icons; expect at least one shelf icon")
			}

			// Obtain the coordinate converter from the touch screen writer.
			displayInfo, err := display.GetInternalInfo(ctx, tconn)
			if err != nil {
				return err
			}
			tcc := tsw.NewTouchCoordConverter(displayInfo.Bounds.Size())
			if err != nil {
				return err
			}

			// Tap on the shelf icon to activate the window. Note that window creation is CPU-consuming. To measure the performance of hotseat background bounds animation more precisely, activating a window instead of creating a window to hide the hotseat.
			centerPoint := scrollableShelfInfo.IconsBoundsInScreen[0].CenterPoint()
			tapPointX, tapPointY := tcc.ConvertLocation(centerPoint)
			if err := stw.Move(tapPointX, tapPointY); err != nil {
				return err
			}
			if err := stw.End(); err != nil {
				return err
			}

			// Verify the hotseat state after hiding.
			if err := ash.WaitForHotseatAnimatingToIdealState(ctx, tconn, ash.ShelfHidden); err != nil {
				return err
			}

			return nil
		}
		if err := hideHotseatByActivatingWindow(ctx, tconn, tsw, stw); err != nil {
			return err
		}

		return nil
	}, hiddenHotseatHistogram)
	if err != nil {
		s.Fatal("Failed to get mean histograms from hiding hotseat by window activation: ", err)
	}

	// Save metrics data from hiding hotseat by window activation.
	for _, h := range histogramGroup {
		mean, err := h.Mean()
		if err != nil {
			s.Fatalf("Failed to get mean for histogram %s: %v", h.Name, err)
		}

		pv.Set(perf.Metric{
			Name:      h.Name + ".WindowActivation",
			Unit:      "percent",
			Direction: perf.BiggerIsBetter,
		}, mean)
	}

	// Save metrics data in file.
	if err := pv.Save(s.OutDir()); err != nil {
		s.Fatal("Failed saving perf data in file: ", err)
	}
}
