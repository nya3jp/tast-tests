// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"strings"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	uiperf "chromiumos/tast/local/bundles/cros/ui/perf"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/touch"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/perfutil"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HotseatScrollPerf,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Records the animation smoothness for shelf scroll animation",
		Contacts: []string{
			"andrewxu@chromium.org",
			"newcomer@chromium.org",
		},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Fixture:      "chromeLoggedInWith100FakeApps",
		Params: []testing.Param{
			{
				Name: "clamshell_mode",
				Val:  false,
			},
			{
				Val: true,
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

func prepareFetchShelfScrollSmoothness(ctx context.Context, tconn *chrome.TestConn, mode uiMode, state uiState) (func(ctx context.Context) error, error) {
	cleanupFuncs := make([]func(ctx context.Context) error, 0, 3)
	cleanupAll := func(ctx context.Context) error {
		var firstErr error
		var errorNum int
		for _, f := range cleanupFuncs {
			if err := f(ctx); err != nil {
				errorNum++
				if firstErr == nil {
					firstErr = err
				}
			}
		}
		if errorNum > 0 {
			return errors.Wrapf(firstErr, "there are %d errors; first error", errorNum)
		}
		return nil
	}

	isInTabletMode := mode == inTabletMode
	isLauncherVisible := state == launcherIsVisible

	if state == overviewIsVisible {
		// Hide notifications before testing overview, so notifications are not shown over the hotseat in  tablet mode.
		if err := ash.CloseNotifications(ctx, tconn); err != nil {
			return cleanupAll, errors.Wrap(err, "failed to close all notifications")
		}

		// Enter overview mode.
		if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
			return cleanupAll, errors.Wrap(err, "failed to enter into the overview mode")
		}

		// Close overview mode after animation smoothness data is collected for it.
		cleanupFuncs = append(cleanupFuncs, func(ctx context.Context) error {
			return ash.SetOverviewModeAndWait(ctx, tconn, false)
		})
	} else if isInTabletMode && !isLauncherVisible {
		// Hide launcher by launching the file app.
		files, err := filesapp.Launch(ctx, tconn)
		if err != nil {
			return cleanupAll, errors.Wrap(err, "failed to hide the home launcher by activating an app")
		}

		// App should be open until the animation smoothness data is collected for in-app shelf.
		cleanupFuncs = append(cleanupFuncs, files.Close)

		tsw, tcc, err := touch.NewTouchscreenAndConverter(ctx, tconn)
		if err != nil {
			return cleanupAll, errors.Wrap(err, "failed to create the touch controller")
		}
		cleanupFuncs = append(cleanupFuncs, func(context.Context) error {
			return tsw.Close()
		})
		stw, err := tsw.NewSingleTouchWriter()
		if err != nil {
			return cleanupAll, errors.Wrap(err, "failed to get the single touch writer")
		}

		// Swipe up the hotseat.
		if err := ash.SwipeUpHotseatAndWaitForCompletion(ctx, tconn, stw, tcc); err != nil {
			return cleanupAll, errors.Wrap(err, "failed to test the in-app shelf")
		}
	} else if !isInTabletMode && isLauncherVisible {
		cleanupFuncs = append(cleanupFuncs, func(ctx context.Context) error {
			return ash.TriggerLauncherStateChange(ctx, tconn, ash.AccelSearch)
		})

		// Show launcher.
		if err := launcher.Open(tconn)(ctx); err != nil {
			return cleanupAll, errors.Wrap(err, "failed to open launcher")
		}
	}

	// Hotseat in different states may have different bounds. So enter shelf overflow mode after tablet/clamshell switch and gesture swipe.
	if err := ash.EnterShelfOverflow(ctx, tconn, false /* underRTL */); err != nil {
		return cleanupAll, err
	}

	if err := ash.WaitForStableShelfBounds(ctx, tconn); err != nil {
		return cleanupAll, errors.Wrap(err, "failed to wait for stable shelf bounds")
	}

	return cleanupAll, nil
}

// HotseatScrollPerf records the animation smoothness for shelf scroll animation.
func HotseatScrollPerf(ctx context.Context, s *testing.State) {
	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	runner := perfutil.NewRunner(cr.Browser())

	var mode uiMode

	if s.Param().(bool) {
		mode = inTabletMode
	} else {
		mode = inClamshellMode
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, s.Param().(bool))
	if err != nil {
		s.Fatalf("Failed to ensure the tablet-mode enabled status to %v: %v", s.Param().(bool), err)
	}
	defer cleanup(ctx)

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
		cleanupFunc, err := prepareFetchShelfScrollSmoothness(ctx, tconn, setting.mode, setting.state)
		if err != nil {
			if err := cleanupFunc(ctx); err != nil {
				s.Error("Failed to cleanup the preparation: ", err)
			}
			s.Fatalf("Failed to prepare for %v: %v", setting.state, err)
		}

		var suffix string
		if setting.state == overviewIsVisible {
			suffix = "OverviewShown"
		}
		runnerError := runner.RunMultiple(ctx, setting.state.String(), uiperf.Run(s, perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
			return runShelfScroll(ctx, tconn)
		}, shelfAnimationHistogramName(setting.mode, setting.state))),
			perfutil.StoreAll(perf.BiggerIsBetter, "percent", suffix))
		if err := cleanupFunc(ctx); err != nil {
			s.Fatalf("Failed to cleanup for %v: %v", setting.state, err)
		}
		if runnerError != nil {
			return
		}
	}

	if err := runner.Values().Save(ctx, s.OutDir()); err != nil {
		s.Fatal("Failed to save performance data in file: ", err)
	}
}
