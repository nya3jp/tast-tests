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
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type hotseatTestType string

const (
	nonOverflow          hotseatTestType = "NonOverflow"
	overflow             hotseatTestType = "Oveflow"              // In test, add enough apps to enter overflow mode.
	showNavigationWidget hotseatTestType = "ShowNavigationWidget" // In test, show the navigation widget (including home button and back button) by disabling the flag which hides the navigation widget as default.
)

type hotseatTestVal struct {
	TestType hotseatTestType
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         HotseatAnimation,
		Desc:         "Measures the framerate of the hotseat animation in tablet mode",
		Contacts:     []string{"newcomer@chromium.org", "manucornet@chromium.org", "andrewxu@chromium.org", "cros-shelf-prod-notifications@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "tablet_mode"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Timeout:      3 * time.Minute,
		Params: []testing.Param{
			{
				Name: "non_overflow_shelf",
				Val:  nonOverflow,
				Pre:  ash.LoggedInWith100DummyApps(),
			},
			{
				Name: "overflow_shelf",
				Val:  overflow,
				Pre:  ash.LoggedInWith100DummyApps(),
			},

			// TODO(https://crbug.com/1083068): when the flag shelf-hide-buttons-in-tablet is removed, delete this sub-test.
			{
				Name: "shelf_with_navigation_widget",
				Val:  showNavigationWidget,
				Pre:  chrome.NewPrecondition("ShowNavigationWidget", chrome.ExtraArgs("--disable-features=HideShelfControlsInTabletMode")),
			},
		},
	})
}

// HotseatAnimation measures the performance of hotseat background bounds animation.
func HotseatAnimation(ctx context.Context, s *testing.State) {
	const (
		// Histograms for hotseat.
		extendedHotseatHistogram                      = "Ash.HotseatTransition.AnimationSmoothness.TransitionToExtendedHotseat"
		extendedHotseatWidgetHistogram                = "Ash.HotseatWidgetAnimation.Widget.AnimationSmoothness.TransitionToExtendedHotseat"
		extendedHotseatTranslucentBackgroundHistogram = "Ash.HotseatWidgetAnimation.TranslucentBackground.AnimationSmoothness.TransitionToExtendedHotseat"
		hiddenHotseatHistogram                        = "Ash.HotseatTransition.AnimationSmoothness.TransitionToHiddenHotseat"
		hiddenHotseatWidgetHistogram                  = "Ash.HotseatWidgetAnimation.Widget.AnimationSmoothness.TransitionToHiddenHotseat"
		shownHotseatHistogram                         = "Ash.HotseatTransition.AnimationSmoothness.TransitionToShownHotseat"
		shownHotseatWidgetHistogram                   = "Ash.HotseatWidgetAnimation.Widget.AnimationSmoothness.TransitionToShownHotseat"
		shownHotseatTranslucentBackgroundHistogram    = "Ash.HotseatWidgetAnimation.TranslucentBackground.AnimationSmoothness.TransitionToShownHotseat"
		shownHomeLauncherHistogram                    = "Apps.HomeLauncherTransition.AnimationSmoothness.FadeInOverview"
		hiddenHomeLauncherHistogram                   = "Apps.HomeLauncherTransition.AnimationSmoothness.FadeOutOverview"

		// Histograms for back button.
		hiddenBackButtonHistogram   = "Ash.NavigationWidget.BackButton.AnimationSmoothness.TransitionToHiddenHotseat"
		shownBackButtonHistogram    = "Ash.NavigationWidget.BackButton.AnimationSmoothness.TransitionToShownHotseat"
		extendedBackButtonHistogram = "Ash.NavigationWidget.BackButton.AnimationSmoothness.TransitionToExtendedHotseat"

		// Histograms for home button.
		hiddenHomeButtonHistogram   = "Ash.NavigationWidget.HomeButton.AnimationSmoothness.TransitionToHiddenHotseat"
		shownHomeButtonHistogram    = "Ash.NavigationWidget.HomeButton.AnimationSmoothness.TransitionToShownHotseat"
		extendedHomeButtonHistogram = "Ash.NavigationWidget.HomeButton.AnimationSmoothness.TransitionToExtendedHotseat"

		// Histograms for navigation widget.
		hiddenWidgetHistogram   = "Ash.NavigationWidget.Widget.AnimationSmoothness.TransitionToHiddenHotseat"
		shownWidgetHistogram    = "Ash.NavigationWidget.Widget.AnimationSmoothness.TransitionToShownHotseat"
		extendedWidgetHistogram = "Ash.NavigationWidget.Widget.AnimationSmoothness.TransitionToExtendedHotseat"

		overviewTimeout = 10 * time.Second
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

	if s.Param().(hotseatTestType) == overflow {
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
	histogramsName := []string{
		hiddenHotseatHistogram,
		hiddenHotseatWidgetHistogram}
	if s.Param().(hotseatTestType) == showNavigationWidget {
		histogramsName = append(histogramsName,
			hiddenBackButtonHistogram,
			hiddenHomeButtonHistogram,
			hiddenWidgetHistogram)
	}
	histogramGroup, err := metrics.RunAndWaitAll(ctx, tconn, time.Second, func() error {
		const numWindows = 1
		conns, err := ash.CreateWindows(ctx, tconn, cr, "", numWindows)
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
	}, histogramsName...)
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
	histogramsName = []string{
		shownHotseatHistogram,
		shownHotseatWidgetHistogram,
		shownHotseatTranslucentBackgroundHistogram,
		extendedHotseatHistogram,
		extendedHotseatWidgetHistogram,
		extendedHotseatTranslucentBackgroundHistogram,
		shownHomeLauncherHistogram,
		hiddenHomeLauncherHistogram}
	if s.Param().(hotseatTestType) == showNavigationWidget {
		histogramsName = append(histogramsName,
			extendedBackButtonHistogram,
			shownBackButtonHistogram,
			extendedHomeButtonHistogram,
			shownHomeButtonHistogram,
			extendedWidgetHistogram,
			shownWidgetHistogram)
	}
	histogramGroup, err = metrics.RunAndWaitAll(ctx, tconn, time.Second, func() error {
		// Add a new tab.
		conn, err := cr.NewConn(ctx, ui.PerftestURL)
		if err != nil {
			return errors.Wrap(err, "cannot create a new tab")
		}
		conn.Close()

		if err := ash.DragToShowOverview(ctx, tsw.Width(), tsw.Height(), stw, tconn); err != nil {
			return errors.Wrap(err, "failed to drag from bottom of the screen to show overview")
		}

		// Enter home launcher from overview by gesture tap.
		pressX := tsw.Width() / 20
		pressY := tsw.Height() / 20
		if err := stw.Swipe(ctx, pressX, pressY, pressX+5, pressY+5, 200*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to tap")
		}
		if err := stw.End(); err != nil {
			return errors.Wrap(err, "failed to finish the tap gesture")
		}
		if err := ash.WaitForOverviewState(ctx, tconn, ash.Hidden, overviewTimeout); err != nil {
			return errors.Wrap(err, "failed to wait for animation to finish")
		}
		if err := ash.WaitForHotseatAnimatingToIdealState(ctx, tconn, ash.ShelfShownHomeLauncher); err != nil {
			return err
		}

		if err := ash.DragToShowOverview(ctx, tsw.Width(), tsw.Height(), stw, tconn); err != nil {
			return errors.Wrap(err, "failed to drag from bottom of the screen to show overview")
		}

		// Enter in-app mode from overview by gesture tap.
		pressX = tsw.Width() / 3
		pressY = tsw.Height() / 3
		if err := stw.Swipe(ctx, pressX, pressY, pressX+5, pressY-5, 200*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to tap")
		}
		if err := stw.End(); err != nil {
			return errors.Wrap(err, "failed to finish the tap gesture")
		}
		if err := ash.WaitForOverviewState(ctx, tconn, ash.Hidden, overviewTimeout); err != nil {
			return errors.Wrap(err, "failed to wait for animation to finish")
		}
		if err := ash.WaitForHotseatAnimatingToIdealState(ctx, tconn, ash.ShelfHidden); err != nil {
			return err
		}

		// Enter home launcher from in-app mode by gesture swipe up from shelf.
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
		if err := ash.WaitForHotseatAnimatingToIdealState(ctx, tconn, ash.ShelfShownHomeLauncher); err != nil {
			return err
		}

		return nil
	}, histogramsName...)
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
	histogramsName = []string{
		hiddenHotseatHistogram,
		hiddenHotseatWidgetHistogram}
	if s.Param().(hotseatTestType) == showNavigationWidget {
		histogramsName = append(histogramsName,
			hiddenBackButtonHistogram,
			hiddenHomeButtonHistogram,
			hiddenWidgetHistogram)
	}
	histogramGroup, err = metrics.RunAndWaitAll(ctx, tconn, time.Second, func() error {
		// Verify the initial hotseat state before hiding.
		if err := ash.WaitForHotseatAnimatingToIdealState(ctx, tconn, ash.ShelfShownHomeLauncher); err != nil {
			return err
		}

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
	}, histogramsName...)
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
