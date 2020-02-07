// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/chrome/ui"
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
type directionType int

const (
	left directionType = iota
	right
)

func scrollAlongOneDirectionUntilEnd(ctx context.Context, tconn *chrome.Conn, d directionType, root *(ui.Node)) error {
	arrowButtonClassName := "ScrollableShelfArrowView"
	icons, err := root.Descendants(ctx, ui.FindParams{ClassName: arrowButtonClassName})
	if err != nil {
		return err
	}

	// Assumes that at the beginning there should be one and only one arrow button.
	if len(icons) != 1 {
		return errors.Errorf("when starting scrolling, there should be one and only arrow button. But there are %d buttons", len(icons))
	}

	arrowButton := icons[0]

	for {
		if err := arrowButton.LeftClick(ctx); err != nil {
			return err
		}

		// Waits enough time for UI refresh.
		if err := testing.Sleep(ctx, 1*time.Second); err != nil {
			return err
		}

		icons, err = root.Descendants(ctx, ui.FindParams{ClassName: arrowButtonClassName})

		// Keeps clicking when there are two arrow buttons showing.
		if len(icons) != 2 {
			break
		}

		// Chooses the suitable arrow button to click based on the scroll direction.
		arrowButton = icons[0]
		x0 := icons[0].Location.Left
		x1 := icons[1].Location.Left
		if (d == left && x0 > x1) || (d == right && x0 < x1) {
			arrowButton = icons[1]
		}
	}

	return nil
}

func runShelfScrollAnimation(ctx context.Context, tconn *chrome.Conn) error {
	// Gets UI root.
	root, err := ui.Root(ctx, tconn)
	if err != nil {
		return err
	}
	defer root.Release(ctx)

	// The best effort to stabilize CPU usage. This may or
	// may not be satisfied in time.
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for system UI to be stabilized")
	}

	if err := scrollAlongOneDirectionUntilEnd(ctx, tconn, right, root); err != nil {
		return err
	}

	// Waits for UI to become stable.
	if err := testing.Sleep(ctx, 1*time.Second); err != nil {
		return err
	}
	if err := scrollAlongOneDirectionUntilEnd(ctx, tconn, left, root); err != nil {
		return err
	}

	return nil
}

func calculateHistogramNameForShelfAnimation(isLauncherVisible bool, isInTabletMode bool) string {
	const (
		baseHistogramName        string = "Apps.ScrollableShelf.AnimationSmoothness"
		clamShellHistogram       string = "ClamshellMode"
		tabletHistogram          string = "TabletMode"
		launcherVisibleHistogram string = "LauncherVisible"
		launcherHiddenHistogram  string = "LauncherHidden"
	)

	histogramName := baseHistogramName

	if isInTabletMode {
		histogramName = strings.Join([]string{histogramName, tabletHistogram}, ".")
	} else {
		histogramName = strings.Join([]string{histogramName, clamShellHistogram}, ".")
	}

	if isLauncherVisible {
		histogramName = strings.Join([]string{histogramName, launcherVisibleHistogram}, ".")
	} else {
		histogramName = strings.Join([]string{histogramName, launcherHiddenHistogram}, ".")
	}

	return histogramName
}

func fetchShelfScrollAnimationSmoothnessHistograms(ctx context.Context, tconn *chrome.Conn, cr *chrome.Chrome, isLauncherVisible bool, isInTabletMode bool) ([]*metrics.Histogram, error) {
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
		// Hides launcher by launching an app.
		app := apps.Chrome
		if err := apps.Launch(ctx, tconn, app.ID); err != nil {
			return nil, errors.Wrapf(err, "failed to launch %s", app.Name)
		}
		if err := ash.WaitForApp(ctx, tconn, app.ID); err != nil {
			return nil, errors.Wrapf(err, "%s did not appear in shelf after launch", app.Name)
		}
	} else if !isInTabletMode && isLauncherVisible {
		// Shows launcher fullscreen.
		if err := ash.TriggerLauncherStateChange(ctx, tconn, ash.AccelShiftSearch); err != nil {
			return nil, errors.Wrap(err, "failed to switch to fullscreen")
		}
		// Verifies the launcher's state.
		if err := ash.WaitForLauncherState(ctx, tconn, launcherTargetState); err != nil {
			return nil, errors.Wrapf(err, "failed to switch the state to %s", launcherTargetState)
		}
	}

	histograms, err := metrics.Run(ctx, cr, func() error {
		if err := runShelfScrollAnimation(ctx, tconn); err != nil {
			return errors.Wrap(err, "fail to run scroll animation")
		}
		return nil
	}, calculateHistogramNameForShelfAnimation(isLauncherVisible, isInTabletMode))
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
		s.Fatal("Unexpected apps in the shelf. Expected only Chrome")
	}

	const pinnedAppNumber = 30

	// Pins additional apps to Shelf.
	installedApps, err := ash.ChromeApps(ctx, tconn)

	if len(installedApps) < pinnedAppNumber {
		s.Fatalf("Number of pinned apps should not be greater than the number of installed apps. However, the actual number of apps to be pinned is %d and the number of installed apps is %d", pinnedAppNumber, len(installedApps))
	}

	for i := 0; i < pinnedAppNumber; i++ {
		if err := ash.PinApp(ctx, tconn, installedApps[i].AppID); err != nil {
			s.Fatalf("Failed to launch %s: %s", installedApps[i].AppID, err)
		}
	}

	pv := perf.NewValues()

	for _, setting := range []struct {
		isLauncherVisible bool
		isInTabletMode    bool
	}{
		{
			isLauncherVisible: false,
			isInTabletMode:    false,
		},
		{
			isLauncherVisible: true,
			isInTabletMode:    false,
		},
		{
			isLauncherVisible: true,
			isInTabletMode:    true,
		},
	} {
		histograms, err := fetchShelfScrollAnimationSmoothnessHistograms(ctx, tconn, cr, setting.isLauncherVisible, setting.isInTabletMode)
		if err != nil {
			s.Fatalf("Failed to run animation: %s with IsLauncherVisible as %t and IsInTabletMode as %t", err, setting.isLauncherVisible, setting.isInTabletMode)
		}

		if err := metrics.StoreHistogramsMeanAsMetricsData(ctx, pv, histograms, perf.Metric{
			Unit:      "percent",
			Direction: perf.BiggerIsBetter,
		}); err != nil {
			s.Fatalf("Failed to store metrics data: %s", err)
		}
	}
}
