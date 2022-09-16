// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	uiperf "chromiumos/tast/local/bundles/cros/ui/perf"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/perfutil"
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
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Measures the framerate of the hotseat animation in tablet mode",
		Contacts:     []string{"newcomer@chromium.org", "manucornet@chromium.org", "andrewxu@chromium.org", "cros-shelf-prod-notifications@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Timeout:      3 * time.Minute,
		Params: []testing.Param{
			{
				Name:    "non_overflow_shelf",
				Val:     nonOverflow,
				Fixture: "chromeLoggedInWith100FakeApps",
			},
			{
				Name:    "overflow_shelf",
				Val:     overflow,
				Fixture: "chromeLoggedInWith100FakeApps",
			},

			// TODO(https://crbug.com/1083068): when the flag shelf-hide-buttons-in-tablet is removed, delete this sub-test.
			{
				Name:    "shelf_with_navigation_widget",
				Val:     showNavigationWidget,
				Fixture: "install100Apps",
			},
		},
	})
}

// HotseatAnimation measures the performance of hotseat background bounds animation.
func HotseatAnimation(ctx context.Context, s *testing.State) {
	const (
		// Histograms for hotseat.
		extendedHotseatWidgetHistogram             = "Ash.HotseatWidgetAnimation.Widget.AnimationSmoothness.TransitionToExtendedHotseat"
		hiddenHotseatHistogram                     = "Ash.HotseatTransition.AnimationSmoothness.TransitionToHiddenHotseat"
		hiddenHotseatWidgetHistogram               = "Ash.HotseatWidgetAnimation.Widget.AnimationSmoothness.TransitionToHiddenHotseat"
		shownHotseatHistogram                      = "Ash.HotseatTransition.AnimationSmoothness.TransitionToShownHotseat"
		shownHotseatWidgetHistogram                = "Ash.HotseatWidgetAnimation.Widget.AnimationSmoothness.TransitionToShownHotseat"
		shownHotseatTranslucentBackgroundHistogram = "Ash.HotseatWidgetAnimation.TranslucentBackground.AnimationSmoothness.TransitionToShownHotseat"
		shownHomeLauncherHistogram                 = "Apps.HomeLauncherTransition.AnimationSmoothness.FadeInOverview"
		hiddenHomeLauncherHistogram                = "Apps.HomeLauncherTransition.AnimationSmoothness.FadeOutOverview"

		// Histograms for back button.
		hiddenBackButtonHistogram = "Ash.NavigationWidget.BackButton.AnimationSmoothness.TransitionToHiddenHotseat"
		shownBackButtonHistogram  = "Ash.NavigationWidget.BackButton.AnimationSmoothness.TransitionToShownHotseat"

		// Histograms for home button.
		hiddenHomeButtonHistogram = "Ash.NavigationWidget.HomeButton.AnimationSmoothness.TransitionToHiddenHotseat"
		shownHomeButtonHistogram  = "Ash.NavigationWidget.HomeButton.AnimationSmoothness.TransitionToShownHotseat"

		// Histograms for navigation widget.
		hiddenWidgetHistogram = "Ash.NavigationWidget.Widget.AnimationSmoothness.TransitionToHiddenHotseat"
		shownWidgetHistogram  = "Ash.NavigationWidget.Widget.AnimationSmoothness.TransitionToShownHotseat"

		overviewTimeout = 10 * time.Second
		swipeDuration   = 500 * time.Millisecond
	)

	var cr *chrome.Chrome

	testType := s.Param().(hotseatTestType)
	if testType == showNavigationWidget {
		opts := []chrome.Option{chrome.DisableFeatures("HideShelfControlsInTabletMode")}
		opts = append(opts, s.FixtValue().([]chrome.Option)...)
		var err error
		cr, err = chrome.New(ctx, opts...)
		if err != nil {
			s.Fatal("Failed to connect to Chrome: ", err)
		}

		defer cr.Close(ctx)
	} else {
		cr = s.FixtValue().(*chrome.Chrome)
	}

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

	// The app list gets shown during transition to tablet mode, Wait for the transition to
	// full screen app list to finish before proceeding with testing hotseat animations to
	// prevent the app list animation's adverse effect on the initial hotseat transition.
	if err := ash.WaitForLauncherState(ctx, tconn, ash.FullscreenAllApps); err != nil {
		s.Fatal("Home launcher failed to show: ", err)
	}

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

	shouldEnterOverflow := testType != nonOverflow
	if shouldEnterOverflow {
		if err := ash.EnterShelfOverflow(ctx, tconn, false /* underRTL */); err != nil {
			s.Fatal(err, "Failed to enter overflow shelf")
		}
	}

	// Changing tablet mode may change the shelf/hotseat bounds. Make sure the bounds
	// stabilize before starting tests.
	if err := ash.WaitForStableShelfBounds(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for stable shelf bouds: ", err)
	}

	runner := perfutil.NewRunner(cr.Browser())

	// Collect metrics data from hiding hotseat by window creation.
	histogramsName := []string{
		hiddenHotseatHistogram,
		hiddenHotseatWidgetHistogram,
		shownHotseatHistogram,
		shownHotseatWidgetHistogram}
	if s.Param().(hotseatTestType) == showNavigationWidget {
		histogramsName = append(histogramsName,
			hiddenBackButtonHistogram,
			hiddenHomeButtonHistogram,
			hiddenWidgetHistogram,
			shownBackButtonHistogram,
			shownHomeButtonHistogram,
			shownWidgetHistogram)
	}
	runner.RunMultiple(ctx, "WindowCreation", uiperf.Run(s, perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
		sctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
		defer cancel()

		conn, err := cr.NewConn(sctx, "", browser.WithNewWindow())
		if err != nil {
			return errors.Wrap(err, "failed to open browser window")
		}
		defer func() {
			if err := conn.Close(); err != nil {
				s.Error("Failed to close a connection: ", err)
			}
		}()
		if err := ash.WaitForHotseatAnimatingToIdealState(ctx, tconn, ash.ShelfHidden); err != nil {
			conn.CloseTarget(ctx)
			return err
		}

		if err := conn.CloseTarget(ctx); err != nil {
			return errors.Wrap(err, "failed to close a target")
		}

		if err := ash.WaitForHotseatAnimatingToIdealState(ctx, tconn, ash.ShelfShownHomeLauncher); err != nil {
			return err
		}

		return nil
	}, histogramsName...)),
		perfutil.StoreAll(perf.BiggerIsBetter, "percent", "WindowCreation"))

	// Collect metrics data from entering/exiting overview.
	histogramsName = []string{
		shownHotseatHistogram,
		shownHotseatWidgetHistogram,
		extendedHotseatWidgetHistogram,
		shownHomeLauncherHistogram,
		hiddenHomeLauncherHistogram}
	if shouldEnterOverflow {
		// Record metrics data which can only be collected in overflow shelf.
		histogramsName = append(histogramsName, shownHotseatTranslucentBackgroundHistogram)
	}
	if testType == showNavigationWidget {
		// Record metrics data which can only be collected with the shelf navigation widget shown.
		histogramsName = append(histogramsName,
			shownBackButtonHistogram,
			shownHomeButtonHistogram,
			shownWidgetHistogram)
	}

	// Histograms for window activation.
	windowActivationHistogramNames := map[string]bool{
		hiddenHotseatHistogram:       true,
		hiddenHotseatWidgetHistogram: true,
	}
	if s.Param().(hotseatTestType) == showNavigationWidget {
		windowActivationHistogramNames[hiddenBackButtonHistogram] = true
		windowActivationHistogramNames[hiddenHomeButtonHistogram] = true
		windowActivationHistogramNames[hiddenWidgetHistogram] = true
	}
	for name := range windowActivationHistogramNames {
		histogramsName = append(histogramsName, name)
	}

	// Add a new tab.
	conn, err := cr.NewConn(ctx, ui.PerftestURL)
	if err != nil {
		s.Fatal("Failed to create a new tab: ", err)
	}
	conn.Close()

	displayInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get display info")
	}

	tcc := tsw.NewTouchCoordConverter(displayInfo.Bounds.Size())
	if err != nil {
		s.Fatal("Failed to generate touch coord converter")
	}

	runner.RunMultiple(ctx, "", uiperf.Run(s, perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
		if err := ash.DragToShowOverview(ctx, tsw, stw, tconn); err != nil {
			return errors.Wrap(err, "failed to drag from bottom of the screen to show overview")
		}

		touchPoint := coords.NewPoint(displayInfo.Bounds.Width/20, displayInfo.Bounds.Height/20)
		// Enter home launcher from overview by gesture tap.
		pressX, pressY := tcc.ConvertLocation(touchPoint)

		if err := stw.Swipe(ctx, pressX, pressY, pressX+5, pressY+5, swipeDuration); err != nil {
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

		if err := ash.DragToShowOverview(ctx, tsw, stw, tconn); err != nil {
			return errors.Wrap(err, "failed to drag from bottom of the screen to show overview")
		}

		// Enter in-app mode from overview by tapping within an overview window bounds.
		window, err := ash.FindFirstWindowInOverview(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "no overview window found")
		}

		touchPoint = window.OverviewInfo.Bounds.CenterPoint()
		pressX, pressY = tcc.ConvertLocation(touchPoint)

		if err := stw.Swipe(ctx, pressX, pressY, pressX+5, pressY-5, swipeDuration); err != nil {
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

		// Swipe the hotseat up from the hidden state to the extended state.
		if err := ash.SwipeUpHotseatAndWaitForCompletion(ctx, tconn, stw, tcc); err != nil {
			return err
		}
		if err := ash.WaitForHotseatAnimatingToIdealState(ctx, tconn, ash.ShelfExtended); err != nil {
			return err
		}

		// Enter home launcher from in-app mode by gesture swipe up from shelf.
		start := displayInfo.Bounds.BottomCenter()
		startX, startY := tcc.ConvertLocation(start)

		end := displayInfo.Bounds.CenterPoint()
		endX, endY := tcc.ConvertLocation(end)

		if err := stw.Swipe(ctx, startX, startY-1, endX, endY, swipeDuration); err != nil {
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
	}, histogramsName...)),
		func(ctx context.Context, pv *perfutil.Values, hists []*metrics.Histogram) error {
			for _, hist := range hists {
				mean, err := hist.Mean()
				if err != nil {
					return errors.Wrapf(err, "failed to get histogram for %s", hist.Name)
				}
				name := hist.Name
				if windowActivationHistogramNames[hist.Name] {
					name = name + ".WindowActivation"
				}
				testing.ContextLog(ctx, name, " = ", mean)
				pv.Append(perf.Metric{
					Name:      name,
					Unit:      "percent",
					Direction: perf.BiggerIsBetter,
				}, mean)
			}
			return nil
		})

	// Save metrics data in file.
	if err := runner.Values().Save(ctx, s.OutDir()); err != nil {
		s.Fatal("Failed saving perf data in file: ", err)
	}
}
