// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LauncherAnimationPerf,
		Desc:         "Measures animation smoothness of lancher animations",
		Contacts:     []string{"mukai@chromium.org", "oshima@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
		Timeout:      3 * time.Minute,
	})
}

func LauncherAnimationPerf(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}
	defer tconn.Close()

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	originalTabletMode, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the tablet mode status: ", err)
	}
	defer ash.SetTabletModeEnabled(ctx, tconn, originalTabletMode)

	// Non Home Launcher is only avaiable in clamshell mode.
	if originalTabletMode {
		ash.SetTabletModeEnabled(ctx, tconn, false)
	}

	// overviewAnimationType specifies the type of the animation of entering to or
	// exiting from the overview mode.
	type launcherAnimationType int
	const (
		animationTypePeeking launcherAnimationType = iota
		animationTypeFullscreenAllApps
		animationTypeFullscreenSearch
		animationTypeHalf
	)

	pv := perf.NewValues()
	currentWindows := 0
	prevHists := map[string]*metrics.Histogram{}
	// Run the launcher open/closet flow for various situations.
	// - change the number of browser windows, 0 or 2
	// - peeking->close, peeking->half, peeking->half->fullscreen->close, fullscreen->close.
	for _, windows := range []int{0, 2} {
		conns, err := ash.CreateWindows(ctx, cr, ui.PerftestURL, windows-currentWindows)
		if err != nil {
			s.Fatal("Failed to create browser windows: ", err)
		}
		defer conns.Close()
		currentWindows = windows
		if err = cpu.WaitUntilIdle(ctx); err != nil {
			s.Error("Failed to wait for system UI to be stabilized: ", err)
		}

		for _, at := range []launcherAnimationType{animationTypePeeking, animationTypeHalf, animationTypeFullscreenSearch, animationTypeFullscreenAllApps} {
			// Wait for 3 seconds to stabilize the result. Note that this doesn't
			// have to be cpu.WaitUntilIdle(). It may wait too much.
			// TODO(mukai): find the way to wait more properly on the idleness of Ash.
			// https://crbug.com/1001314.
			trigger := ash.AccelSearch
			state := ash.Peeking
			shouldType := false
			switch at {
			case animationTypeFullscreenAllApps:
				trigger = ash.AccelShiftSearch
				state = ash.FullscreenAllApps
				break
			case animationTypeFullscreenSearch:
				shouldType = true
				break
			case animationTypeHalf:
				shouldType = true
				break
			}

			if err = testing.Sleep(ctx, 3*time.Second); err != nil {
				s.Fatal("Failed to wait: ", err)
			}

			if err = ash.TriggerLauncherStateChangeAndWait(ctx, tconn, trigger, state); err != nil {
				s.Fatal("Failed to open launcher: ", err)
			}
			if shouldType {
				if err := kb.Type(ctx, "abcde"); err != nil {
					s.Fatal(err, "failed to type a", err)
				}
				if err = ash.WaitForLauncherState(ctx, tconn, ash.Half); err != nil {
					s.Fatal("Failed to open launcher: ", err)
				}
			}
			if at == animationTypeFullscreenSearch {
				if err = ash.TriggerLauncherStateChangeAndWait(ctx, tconn, ash.AccelShiftSearch, ash.FullscreenSearch); err != nil {
					s.Fatal("Failed to open launcher: ", err)
				}
			}
			// Close
			if err = ash.TriggerLauncherStateChangeAndWait(ctx, tconn, ash.AccelSearch, ash.Closed); err != nil {
				s.Fatal("Failed to close launcher: ", err)
			}

			var suffix string
			switch at {
			case animationTypePeeking:
				suffix = "Peeking.ClamshellMode"
				break
			case animationTypeFullscreenAllApps:
				suffix = "FullscreenAllApps.ClamshellMode"
				break
			case animationTypeFullscreenSearch:
				suffix = "FullscreenSearch.ClamshellMode"
				break
			case animationTypeHalf:
				suffix = "Half.ClamshellMode"
				break
			}

			for _, histName := range []string{
				"Apps.StateTransition.AnimationSmoothness." + suffix,
				"Apps.StateTransition.AnimationSmoothness.Close.ClamshellMode",
			} {
				histogram, err := metrics.GetHistogram(ctx, cr, histName)
				if err != nil {
					s.Fatalf("Failed to get histogram %s: %v", histName, err)
				}
				histToReport := histogram
				if prevHist, ok := prevHists[histName]; ok {
					if histToReport, err = histogram.Diff(prevHist); err != nil {
						s.Fatalf("Failed to compute the histogram diff of %s: %v", histName, err)
					}
				}
				prevHists[histName] = histogram
				pv.Set(perf.Metric{
					Name:      fmt.Sprintf("%s.%dwindows", histName, currentWindows),
					Unit:      "percent",
					Direction: perf.BiggerIsBetter,
				}, histToReport.Mean())
			}
		}

		if err := pv.Save(s.OutDir()); err != nil {
			s.Error("Failed saving perf data: ", err)
		}
	}
}
