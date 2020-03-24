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

func init() {
	testing.AddTest(&testing.Test{
		Func:         HotseatAnimation,
		Desc:         "Measures the framerate of the hotseat animation in tablet mode",
		Contacts:     []string{"newcomer@chromium.org", "manucornet@chromium.org", "cros-shelf-prod-notifications@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "tablet_mode"},
		Pre:          chrome.LoggedIn(),
		Timeout:      8 * time.Minute,
	})
}

const (
	extendedHotseatHistogram = "Ash.HotseatTransition.AnimationSmoothness.TransitionToExtendedHotseat"
	hiddenHotseatHistogram   = "Ash.HotseatTransition.AnimationSmoothness.TransitionToHiddenHotseat"
	shownHotseatHistogram    = "Ash.HotseatTransition.AnimationSmoothness.TransitionToShownHotseat"
)

func hideHotseatByActivatingWindow(ctx context.Context, tconn *chrome.TestConn, tsw *input.TouchscreenEventWriter, stw *input.SingleTouchEventWriter) error {
	const errorMsg = "failed to hide hotseat by activating a window"

	// Verify the initial hotseat state before hiding.
	if err := ash.WaitForHotseatAnimatingToIdealState(ctx, tconn, ash.ShelfShownHomeLauncher); err != nil {
		return errors.Wrap(err, errorMsg)
	}

	scrollableShelfInfo, err := ash.FetchScrollableShelfInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, errorMsg)
	}

	if len(scrollableShelfInfo.IconsBoundsInScreen) == 0 {
		return errors.Errorf("%s: got 0 shelf icons; expect at least one shelf icon", errorMsg)
	}

	// Obtain the coordinate converter from the touch screen writer.
	displayInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, errorMsg)
	}
	tcc := tsw.NewTouchCoordConverter(displayInfo.Bounds.Size())
	if err != nil {
		return errors.Wrap(err, errorMsg)
	}

	// Tap on the shelf icon to activate the window. Note that window creation is CPU-consuming. To measure the performance of hotseat background bounds animation more precisely, activating a window instead of creating a window to hide the hotseat.
	centerPoint := scrollableShelfInfo.IconsBoundsInScreen[0].CenterPoint()
	tapPointX, tapPointY := tcc.ConvertLocation(centerPoint)
	if err := stw.Move(tapPointX, tapPointY); err != nil {
		return errors.Wrap(err, errorMsg)
	}
	if err := stw.End(); err != nil {
		return errors.Wrap(err, errorMsg)
	}

	// Verify the hotseat state after hiding.
	if err := ash.WaitForHotseatAnimatingToIdealState(ctx, tconn, ash.ShelfHidden); err != nil {
		return errors.Wrap(err, errorMsg)
	}

	return nil
}

func addNewTab(ctx context.Context, cr *chrome.Chrome, url string) error {
	conn, err := cr.NewConn(ctx, url)
	if err != nil {
		return errors.Wrap(err, "cannot create a new tab")
	}

	conn.Close()
	return nil
}

// HotseatAnimation measures the performance of hotseat background bounds animation.
func HotseatAnimation(ctx context.Context, s *testing.State) {
	const errorMsg = "Failed to collect data for hotseat bounds animation: "

	cr := s.PreValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal(errorMsg, err)
	}

	orientation, err := display.GetOrientation(ctx, tconn)
	if err != nil {
		s.Fatal(errorMsg, err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		s.Fatal(errorMsg, err)
	}
	defer cleanup(ctx)

	// Prepare the touch screen as this test requires touch scroll events.
	tsw, err := input.Touchscreen(ctx)
	if err != nil {
		s.Fatal(errorMsg, err)
	}
	defer tsw.Close()
	if err := tsw.SetRotation(-orientation.Angle); err != nil {
		s.Fatal(errorMsg, err)
	}

	stw, err := tsw.NewSingleTouchWriter()
	if err != nil {
		s.Fatal(errorMsg, err)
	}
	defer stw.Close()

	// Wait for the animations to complete and for things to settle down.
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal(errorMsg, err)
	}

	// Collect metrics data for hiding hotseat by window creation.
	histogramGroup1, err := metrics.Run(ctx, tconn, func() error {
		const numWindows = 1
		conns, err := ash.CreateWindows(ctx, cr, ui.EmptyURL, numWindows)
		if err != nil {
			return err
		}
		if err := conns.Close(); err != nil {
			return err
		}

		if err := ash.WaitForHotseatAnimatingToIdealState(ctx, tconn, ash.ShelfHidden); err != nil {
			return err
		}

		return nil
	}, hiddenHotseatHistogram)
	if err != nil {
		s.Fatal(errorMsg, err)
	}
	for _, h := range histogramGroup1 {
		if h.Name == hiddenHotseatHistogram {
			h.Name = hiddenHotseatHistogram + ".WindowCreation"
		}
	}

	// Collect metrics data for entering/exiting overview.
	histogramGroup2, err := metrics.Run(ctx, tconn, func() error {
		if err := addNewTab(ctx, cr, ui.PerftestURL); err != nil {
			return err
		}

		if err := ash.DragToShowOverview(ctx, tsw.Width(), tsw.Height(), stw, tconn); err != nil {
			return errors.Wrap(err, "failed to drag from bottom of the screen to show overview")
		}

		if err := ash.WaitForHotseatAnimatingToIdealState(ctx, tconn, ash.ShelfExtended); err != nil {
			return err
		}

		pressX := tsw.Width() * 5 / 6
		pressY := tsw.Height() / 2
		if err := stw.Swipe(ctx, pressX, pressY, pressX+5, pressY-5, 200*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to tap")
		}
		if err := stw.End(); err != nil {
			return errors.Wrap(err, "failed to finish the tap gesture")
		}

		if err := ash.WaitForOverviewState(ctx, tconn, ash.Hidden); err != nil {
			return errors.Wrap(err, "failed to wait for animation to finish")
		}

		if err := ash.WaitForHotseatAnimatingToIdealState(ctx, tconn, ash.ShelfShownHomeLauncher); err != nil {
			return err
		}

		if err := ash.DragToShowOverview(ctx, tsw.Width(), tsw.Height(), stw, tconn); err != nil {
			return errors.Wrap(err, "failed to drag from bottom of the screen to show overview")
		}

		if err := ash.WaitForHotseatAnimatingToIdealState(ctx, tconn, ash.ShelfExtended); err != nil {
			return err
		}

		pressX = tsw.Width() / 3
		pressY = tsw.Height() / 3

		if err := stw.Swipe(ctx, pressX, pressY, pressX+5, pressY-5, 200*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to tap")
		}
		if err := stw.End(); err != nil {
			return errors.Wrap(err, "failed to finish the tap gesture")
		}

		if err := ash.WaitForOverviewState(ctx, tconn, ash.Hidden); err != nil {
			return errors.Wrap(err, "failed to wait for animation to finish")
		}

		startX := tsw.Width() / 2
		startY := tsw.Height() - 1

		endX := startX
		endY := tsw.Height() / 2

		if err := stw.Swipe(ctx, startX, startY, endX, endY, 200*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to swipe")
		}

		if err := stw.End(); err != nil {
			return errors.Wrap(err, "failed to finish the swipe gesture")
		}

		if err := ash.WaitForLauncherState(ctx, tconn, ash.FullscreenAllApps); err != nil {
			return errors.Wrap(err, "home launcher failed to show")
		}

		return nil
	},
		shownHotseatHistogram,
		extendedHotseatHistogram)
	if err != nil {
		s.Fatal(errorMsg, err)
	}

	// Collect metrics data for hiding hotseat by window activation.
	histogramGroup3, err := metrics.Run(ctx, tconn, func() error {
		if err := hideHotseatByActivatingWindow(ctx, tconn, tsw, stw); err != nil {
			return err
		}
		return nil
	}, hiddenHotseatHistogram)
	if err != nil {
		s.Fatal(errorMsg, err)
	}
	for _, h := range histogramGroup3 {
		if h.Name == hiddenHotseatHistogram {
			h.Name = hiddenHotseatHistogram + ".WindowActivation"
		}
	}

	// Save metrics data.
	pv := perf.NewValues()
	for _, h := range append(append(histogramGroup1, histogramGroup2...), histogramGroup3...) {
		mean, err := h.Mean()
		if err != nil {
			s.Fatalf("%sFailed to get mean for histogram %s: %v", errorMsg, h.Name, err)
		}

		pv.Set(perf.Metric{
			Name:      h.Name,
			Unit:      "percent",
			Direction: perf.BiggerIsBetter,
		}, mean)
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
