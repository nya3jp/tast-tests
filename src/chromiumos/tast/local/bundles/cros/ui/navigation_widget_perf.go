// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         NavigationWidgetPerf,
		Desc:         "Measures the framerate of the navigation widget animation in tablet mode",
		Contacts:     []string{"andrewxu@chromium.org", "newcomer@chromium.org"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "tablet_mode"},
		Timeout:      8 * time.Minute,
	})
}

// NavigationWidgetPerf measures the performance of navigation widget animation.
func NavigationWidgetPerf(ctx context.Context, s *testing.State) {
	const (
		hiddenBackButtonHistogram   = "Ash.NavigationWidget.BackButton.AnimationSmoothness.TransitionToHiddenHotseat"
		shownBackButtonHistogram    = "Ash.NavigationWidget.BackButton.AnimationSmoothness.TransitionToShownHotseat"
		extendedBackButtonHistogram = "Ash.NavigationWidget.BackButton.AnimationSmoothness.TransitionToExtendedHotseat"

		hiddenHomeButtonHistogram   = "Ash.NavigationWidget.HomeButton.AnimationSmoothness.TransitionToHiddenHotseat"
		shownHomeButtonHistogram    = "Ash.NavigationWidget.HomeButton.AnimationSmoothness.TransitionToShownHotseat"
		extendedHomeButtonHistogram = "Ash.NavigationWidget.HomeButton.AnimationSmoothness.TransitionToExtendedHotseat"

		hiddenWidgetHistogram   = "Ash.NavigationWidget.Widget.AnimationSmoothness.TransitionToHiddenHotseat"
		shownWidgetHistogram    = "Ash.NavigationWidget.Widget.AnimationSmoothness.TransitionToShownHotseat"
		extendedWidgetHistogram = "Ash.NavigationWidget.Widget.AnimationSmoothness.TransitionToExtendedHotseat"
	)

	// Shelf navigation widget is not shown as default. So disable the feature which hides the shelf navigation widget.
	cr, err := chrome.New(ctx, chrome.ExtraArgs("--disable-features=HideShelfControlsInTabletMode"))
	if err != nil {
		s.Fatal("Failed to disable the flag HideShelfControlsInTabletMode: ", err)
	}

	var tconn *chrome.TestConn
	tconn, err = cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		s.Fatal("Failed to ensure in tablet mode: ", err)
	}
	defer cleanup(ctx)

	// Prepare for touch events.
	orientation, err := display.GetOrientation(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the display rotation: ", err)
	}
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

	pv := perf.NewValues()

	// Wait for the animations to complete and for things to settle down.
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed waiting for CPU to become idle: ", err)
	}

	// Fetch histograms from entering/exiting overview mode.
	var histograms []*metrics.Histogram
	histograms, err = metrics.Run(ctx, tconn, func() error {
		if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
			s.Fatal("Failed to enter into the overview mode: ", err)
		}

		if err := ash.WaitForHotseatAnimatingToIdealState(ctx, tconn, ash.ShelfExtended); err != nil {
			return err
		}

		if err := ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
			s.Fatal("Failed to exit the overview mode: ", err)
		}

		if err := ash.WaitForHotseatAnimatingToIdealState(ctx, tconn, ash.ShelfShownHomeLauncher); err != nil {
			return err
		}

		return nil
	},
		// Back button histograms
		extendedBackButtonHistogram,
		shownBackButtonHistogram,

		// Home button histograms
		extendedHomeButtonHistogram,
		shownHomeButtonHistogram,

		// Navigation widget histograms
		extendedWidgetHistogram,
		shownWidgetHistogram,
	)
	if err != nil {
		s.Fatal("Failed to get mean histograms from entering/exiting overview: ", err)
	}

	for _, h := range histograms {
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

	// Fetch histograms from creating a window to hide UI components.
	histograms, err = metrics.Run(ctx, tconn, func() error {
		conns, err := ash.CreateWindows(ctx, cr, "", 1)
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
	},
		hiddenBackButtonHistogram,
		hiddenHomeButtonHistogram,
		hiddenWidgetHistogram,
	)
	if err != nil {
		s.Fatal("Failed to get mean histograms from creating a window: ", err)
	}

	for _, h := range histograms {
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

	if err := ash.DragToShowHomescreen(ctx, tsw.Width(), tsw.Height(), stw, tconn); err != nil {
		s.Fatal("Failed to show home launcher: ", err)
	}

	if err := ash.WaitForHotseatAnimatingToIdealState(ctx, tconn, ash.ShelfShownHomeLauncher); err != nil {
		s.Fatal("Failed to wait for the expected hotseat state: ", err)
	}

	// Fetch histograms from activating a window to hide UI components.
	histograms, err = metrics.Run(ctx, tconn, func() error {
		scrollableShelfInfo, err := ash.FetchScrollableShelfInfoForState(ctx, tconn, &ash.ShelfState{})
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

		// Tap on the shelf icon to activate a window to hide the hotseat.
		centerPoint := scrollableShelfInfo.IconsBoundsInScreen[0].CenterPoint()
		tapPointX, tapPointY := tcc.ConvertLocation(centerPoint)
		if err := stw.Move(tapPointX, tapPointY); err != nil {
			return err
		}
		if err := stw.End(); err != nil {
			return err
		}

		// Verify the final hotseat state.
		if err := ash.WaitForHotseatAnimatingToIdealState(ctx, tconn, ash.ShelfHidden); err != nil {
			return err
		}

		return nil
	},
		hiddenBackButtonHistogram,
		hiddenHomeButtonHistogram,
		hiddenWidgetHistogram,
	)
	if err != nil {
		s.Fatal("Failed to get mean histograms from activating a window: ", err)
	}

	for _, h := range histograms {
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
