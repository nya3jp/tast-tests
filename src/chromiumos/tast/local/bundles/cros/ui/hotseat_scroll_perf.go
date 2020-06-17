// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/chrome/ui/pointer"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
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
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Pre:          ash.LoggedInWith100DummyApps(),
		Params: []testing.Param{
			{
				Name: "clamshell_mode",
				Val:  false,
			},
			{
				ExtraSoftwareDeps: []string{"tablet_mode"},
				Val:               true,
			},
		},
	})
}

// direction specifies the scroll direction.
type direction int

const (
	scrollToLeft direction = iota
	scrollToRight
)

// uiMode specifies whether it is in clamshell mode or tablet mode.
type uiMode int

const (
	inClamshellMode uiMode = iota
	inTabletMode
)

func (mode uiMode) String() string {
	const (
		clamShellHistogram string = "ClamshellMode"
		tabletHistogram    string = "TabletMode"
	)

	switch mode {
	case inClamshellMode:
		return clamShellHistogram
	case inTabletMode:
		return tabletHistogram
	default:
		return "unknown"
	}
}

type uiState int

const (
	launcherIsVisible uiState = iota
	launcherIsHidden
	overviewIsVisible
)

func (state uiState) String() string {
	const (
		launcherVisibleHistogram string = "LauncherVisible"
		launcherHiddenHistogram  string = "LauncherHidden"
	)

	switch state {
	case launcherIsVisible:
		return launcherVisibleHistogram
	case launcherIsHidden:
		return launcherHiddenHistogram
	case overviewIsVisible:
		// When overview is visible, return histogram for launcher hidden, since
		// no metric exists for overview mode.
		return launcherHiddenHistogram
	default:
		return "unknown"
	}
}

func scrollToEnd(ctx context.Context, tconn *chrome.TestConn, d direction) error {
	var scrollCount int

	for {
		// Calculate the suitable scroll offset to go to a new shelf page.
		info, err := ash.FetchScrollableShelfInfoForState(ctx, tconn, &ash.ShelfState{})
		if err != nil {
			return err
		}
		var pageOffset float32
		if d == scrollToLeft {
			pageOffset = -info.PageOffset
		} else {
			pageOffset = info.PageOffset
		}

		// Calculate the target scroll offset based on pageOffset.
		if info, err = ash.FetchScrollableShelfInfoForState(ctx, tconn, &ash.ShelfState{ScrollDistance: pageOffset}); err != nil {
			return err
		}

		// Choose the arrow button to be clicked based on the scroll direction.
		var arrowBounds coords.Rect
		if d == scrollToLeft {
			arrowBounds = info.LeftArrowBounds
		} else {
			arrowBounds = info.RightArrowBounds
		}
		if arrowBounds.Width == 0 {
			// Have scrolled to the end. Break the loop.
			break
		}

		if err := ash.ScrollShelfAndWaitUntilFinish(ctx, tconn, arrowBounds, info.TargetMainAxisOffset); err != nil {
			return err
		}

		scrollCount = scrollCount + 1
	}

	if scrollCount == 0 {
		return errors.New("Scroll animation should be triggered at least one time in the loop")
	}

	return nil
}

func runShelfScroll(ctx context.Context, tconn *chrome.TestConn) error {
	// The best effort to stabilize CPU usage. This may or
	// may not be satisfied in time.
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for system UI to be stabilized")
	}

	if err := scrollToEnd(ctx, tconn, scrollToRight); err != nil {
		return err
	}

	if err := scrollToEnd(ctx, tconn, scrollToLeft); err != nil {
		return err
	}

	return nil
}

func shelfAnimationHistogramName(mode uiMode, state uiState) string {
	const baseHistogramName = "Apps.ScrollableShelf.AnimationSmoothness"
	comps := []string{baseHistogramName, mode.String(), state.String()}
	return strings.Join(comps, ".")
}

func fetchShelfScrollSmoothnessHistogram(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, mode uiMode, state uiState) ([]*metrics.Histogram, error) {
	isInTabletMode := mode == inTabletMode
	isLauncherVisible := state == launcherIsVisible

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, isInTabletMode)
	if err != nil {
		return nil, errors.Wrap(err, "failed to ensure in clamshell mode")
	}
	defer cleanup(ctx)

	launcherTargetState := ash.Closed
	if isLauncherVisible {
		launcherTargetState = ash.FullscreenAllApps
	}

	if state == overviewIsVisible {
		// Hide notifications before testing overview, so notifications are not shown over the hotseat in  tablet mode.
		if err := ash.HideAllNotifications(ctx, tconn); err != nil {
			return nil, errors.Wrap(err, "failed to hide all notifications")
		}

		// Enter overview mode.
		if err = ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
			return nil, errors.Wrap(err, "failed to enter into the overview mode")
		}

		// Close overview mode after animation smoothness data is collected for it.
		defer ash.SetOverviewModeAndWait(ctx, tconn, false)
	} else if isInTabletMode && !isLauncherVisible {
		// Hide launcher by launching the file app.
		files, err := filesapp.Launch(ctx, tconn)
		if err != nil {
			return nil, errors.Wrap(err, "failed to hide the home launcher by activating an app")
		}

		// App should be open until the animation smoothness data is collected for in-app shelf.
		defer files.Close(ctx)

		tc, err := pointer.NewTouchController(ctx, tconn)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create the touch controller")
		}
		defer tc.Close()

		// Swipe up the hotseat.
		if err := ash.SwipeUpHotseatAndWaitForCompletion(ctx, tconn, tc.EventWriter(), tc.TouchCoordConverter()); err != nil {
			return nil, errors.Wrap(err, "failed to test the in-app shelf")
		}
	} else if !isInTabletMode && isLauncherVisible {
		// Show launcher fullscreen.
		if err := ash.TriggerLauncherStateChange(ctx, tconn, ash.AccelShiftSearch); err != nil {
			return nil, errors.Wrap(err, "failed to switch to fullscreen")
		}
		defer ash.TriggerLauncherStateChange(ctx, tconn, ash.AccelSearch)
		// Verify the launcher's state.
		if err := ash.WaitForLauncherState(ctx, tconn, launcherTargetState); err != nil {
			return nil, errors.Wrapf(err, "failed to switch the state to %s", launcherTargetState)
		}
	}

	// Hotseat in different states may have different bounds. So enter shelf overflow mode after tablet/clamshell switch and gesture swipe.
	if err := ash.EnterShelfOverflow(ctx, tconn); err != nil {
		return nil, err
	}

	histograms, err := metrics.RunAndWaitAll(ctx, tconn, time.Second, func() error {
		if err := runShelfScroll(ctx, tconn); err != nil {
			return errors.Wrap(err, "fail to run scroll animation")
		}
		return nil
	}, shelfAnimationHistogramName(mode, state))
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
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	defer ui.WaitForLocationChangeCompleted(ctx, tconn)

	pv := perf.NewValues()

	var mode uiMode

	if s.Param().(bool) {
		mode = inTabletMode
	} else {
		mode = inClamshellMode
	}

	type testSetting struct {
		state uiState
		mode  uiMode
	}

	settings := []testSetting{
		{
			state: launcherIsHidden,
			mode:  mode,
		},
		{
			state: overviewIsVisible,
			mode:  mode,
		},
		{
			state: launcherIsVisible,
			mode:  mode,
		},
	}

	for _, setting := range settings {
		histograms, err := fetchShelfScrollSmoothnessHistogram(ctx, cr, tconn, setting.mode, setting.state)
		if err != nil {
			s.Fatalf("Failed to run animation with ui mode as %s, display state as %s: %v", setting.mode, setting.state, err)
		}

		// Save metrics data for scrolling the shelf.
		for _, h := range histograms {
			mean, err := h.Mean()
			if err != nil {
				s.Fatalf("Failed to get mean for histogram %s: %v", h.Name, err)
			}

			if setting.state == overviewIsVisible {
				h.Name = h.Name + ".OverviewShown"
			}
			s.Log(h.Name, "= ", mean)

			pv.Set(perf.Metric{
				Name:      h.Name,
				Unit:      "percent",
				Direction: perf.BiggerIsBetter,
			}, mean)
		}
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to save performance data in file: ", err)
	}
}
