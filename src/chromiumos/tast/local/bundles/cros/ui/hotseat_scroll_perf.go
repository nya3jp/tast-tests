// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: HotseatScrollPerf,
		Desc: "Records the animation smoothness for shelf scroll animation",
		Contacts: []string{
			"andrewxu@chromium.org",
			"newcomer@chromium.org",
		},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Pre:          ash.LoggedInWith100DummyApps(),
	})
}

// directionType specifies the scroll direction.
type scrollDirectionType int

const (
	scrollToLeft scrollDirectionType = iota
	scrollToRight
)

type uiMode string

const (
	inTabletMode    uiMode = "InTabletMode"
	inClamshellMode uiMode = "InClamshellMode"
)

type launcherState string

const (
	launcherIsVisible launcherState = "LauncherIsVisible"
	launcherIsHidden  launcherState = "LauncherIsHidden"
)

func scrollAlongOneDirectionUntilEnd(ctx context.Context, tconn *chrome.TestConn, d scrollDirectionType) error {
	var scrollCount int

	for {
		var err error
		var info *ash.ScrollableShelfInfo

		// Calculate the suitable scroll offset to go to a new shelf page.
		if info, err = ash.FetchScrollableShelfInfo(ctx, tconn, 0); err != nil {
			return err
		}
		var pageOffset float32
		if d == scrollToLeft {
			pageOffset = -info.PageOffset
		} else {
			pageOffset = info.PageOffset
		}

		// Calculated the target scroll offset based on |pageOffset|.
		if info, err = ash.FetchScrollableShelfInfo(ctx, tconn, pageOffset); err != nil {
			return err
		}

		// Choose the arrow button to be clicked based on the scroll direction.
		var arrowBounds ash.Rect
		if d == scrollToLeft {
			arrowBounds = info.LeftArrowBounds
		} else {
			arrowBounds = info.RightArrowBounds
		}
		if arrowBounds.Width == 0 {
			// Have scrolled to the end. Break the loop.
			break
		}

		if err = ash.PressShelfArrowButtonAndWaitForCompletion(ctx, tconn, arrowBounds, info.TargetMainAxisOffset); err != nil {
			return err
		}

		scrollCount = scrollCount + 1
	}

	if scrollCount == 0 {
		return errors.New("Scroll animation should be triggered at least one time in the loop")
	}

	return nil
}

func runShelfScrollAnimation(ctx context.Context, tconn *chrome.TestConn) error {
	// The best effort to stabilize CPU usage. This may or
	// may not be satisfied in time.
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for system UI to be stabilized")
	}

	if err := scrollAlongOneDirectionUntilEnd(ctx, tconn, scrollToRight); err != nil {
		return err
	}

	if err := scrollAlongOneDirectionUntilEnd(ctx, tconn, scrollToLeft); err != nil {
		return err
	}

	return nil
}

func shelfAnimationHistogramName(launcherVisibility launcherState, mode uiMode) string {
	const (
		baseHistogramName        string = "Apps.ScrollableShelf.AnimationSmoothness"
		clamShellHistogram       string = "ClamshellMode"
		tabletHistogram          string = "TabletMode"
		launcherVisibleHistogram string = "LauncherVisible"
		launcherHiddenHistogram  string = "LauncherHidden"
	)

	isInTabletMode := mode == inTabletMode
	isLauncherVisible := launcherVisibility == launcherIsVisible

	comps := []string{baseHistogramName}
	if isInTabletMode {
		comps = append(comps, tabletHistogram)
	} else {
		comps = append(comps, clamShellHistogram)
	}
	if isLauncherVisible {
		comps = append(comps, launcherVisibleHistogram)
	} else {
		comps = append(comps, launcherHiddenHistogram)
	}

	return strings.Join(comps, ".")
}

func fetchShelfScrollAnimationSmoothnessHistograms(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome, launcherVisbility launcherState, mode uiMode) ([]*metrics.Histogram, error) {
	isInTabletMode := mode == inTabletMode
	isLauncherVisible := launcherVisbility == launcherIsVisible

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, isInTabletMode)
	if err != nil {
		return nil, errors.Wrap(err, "failed to ensure in clamshell mode")
	}
	defer cleanup(ctx)

	launcherTargetState := ash.Closed
	if isLauncherVisible {
		launcherTargetState = ash.FullscreenAllApps
	}

	if isInTabletMode && !isLauncherVisible {
		// Hide launcher by launching an app.
		app := apps.Chrome
		if err := apps.Launch(ctx, tconn, app.ID); err != nil {
			return nil, errors.Wrapf(err, "failed to launch %s", app.Name)
		}
		if err := ash.WaitForApp(ctx, tconn, app.ID); err != nil {
			return nil, errors.Wrapf(err, "%s did not appear in shelf after launch", app.Name)
		}
	} else if !isInTabletMode && isLauncherVisible {
		// Show launcher fullscreen.
		if err := ash.TriggerLauncherStateChange(ctx, tconn, ash.AccelShiftSearch); err != nil {
			return nil, errors.Wrap(err, "failed to switch to fullscreen")
		}
		// Verify the launcher's state.
		if err := ash.WaitForLauncherState(ctx, tconn, launcherTargetState); err != nil {
			return nil, errors.Wrapf(err, "failed to switch the state to %s", launcherTargetState)
		}
	}

	histograms, err := metrics.Run(ctx, cr, func() error {
		if err := runShelfScrollAnimation(ctx, tconn); err != nil {
			return errors.Wrap(err, "fail to run scroll animation")
		}
		return nil
	}, shelfAnimationHistogramName(launcherVisbility, mode))
	if err != nil {
		return nil, errors.Wrap(err, "failed to run scroll animation or get histograms")
	}

	return histograms, nil
}

// HotseatScrollPerf records the animation smoothness for shelf scroll animation.
func HotseatScrollPerf(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// At login, we should have just Chrome in the Shelf.
	if shelfItems, err := ash.ShelfItems(ctx, tconn); err != nil {
		s.Fatal("Failed to get shelf items: ", err)
	} else if len(shelfItems) != 1 {
		s.Fatalf("Unexpected num of apps in the shelf: got %d; want %t", len(shelfItems), 1)
	}

	const pinnedAppNumber = 30

	// Pin additional apps to Shelf.
	installedApps, err := ash.ChromeApps(ctx, tconn)

	if len(installedApps) < pinnedAppNumber {
		s.Fatalf("Unexpected number of pinned apps: got %d; want less than %d which is the number of installed apps", pinnedAppNumber, len(installedApps))
	}

	for idx, app := range installedApps {
		if idx == pinnedAppNumber {
			break
		}

		if err := ash.PinApp(ctx, tconn, app.AppID); err != nil {
			s.Fatalf("Failed to launch %s: %s", app.AppID, err)
		}
	}

	pv := perf.NewValues()

	for _, setting := range []struct {
		launcherVisibility launcherState
		mode               uiMode
	}{
		{
			launcherVisibility: launcherIsHidden,
			mode:               inClamshellMode,
		},
		{
			launcherVisibility: launcherIsVisible,
			mode:               inClamshellMode,
		},
		{
			launcherVisibility: launcherIsVisible,
			mode:               inTabletMode,
		},
	} {
		histograms, err := fetchShelfScrollAnimationSmoothnessHistograms(ctx, tconn, cr, setting.launcherVisibility, setting.mode)
		if err != nil {
			s.Fatalf("Failed to run animation with launcher visibility as %s and ui mode as %s: %s", setting.launcherVisibility, setting.mode, err)
		}

		if err := metrics.StoreHistogramsMeanAsMetricsData(ctx, pv, histograms, perf.Metric{
			Unit:      "percent",
			Direction: perf.BiggerIsBetter,
		}); err != nil {
			s.Fatalf("Failed to store metrics data: %s", err)
		}
	}
}
