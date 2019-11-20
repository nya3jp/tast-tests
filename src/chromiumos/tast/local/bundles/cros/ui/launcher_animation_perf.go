// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
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

// launcherAnimationType specifies the type of the animation of opening
// launcher.
type launcherAnimationType int

const (
	animationTypePeeking launcherAnimationType = iota
	animationTypeFullscreenAllApps
	animationTypeFullscreenSearch
	animationTypeHalf
)

func runLauncherAnimation(ctx context.Context, tconn *chrome.Conn, kb *input.KeyboardEventWriter, at launcherAnimationType) error {
	trigger := ash.AccelSearch
	firstState := ash.Peeking
	if at == animationTypeFullscreenAllApps {
		trigger = ash.AccelShiftSearch
		firstState = ash.FullscreenAllApps
	}
	if err := ash.TriggerLauncherStateChange(ctx, tconn, trigger); err != nil {
		return errors.Wrap(err, "failed to open launcher")
	}
	if err := ash.WaitForLauncherState(ctx, tconn, firstState); err != nil {
		return errors.Wrap(err, "failed to wait for state")
	}

	if at == animationTypeHalf || at == animationTypeFullscreenSearch {
		if err := kb.Type(ctx, "a"); err != nil {
			return errors.Wrap(err, "failed to type 'a'")
		}
		if err := ash.WaitForLauncherState(ctx, tconn, ash.Half); err != nil {
			return errors.Wrap(err, "failed to switch the state to 'Half'")
		}
	}

	if at == animationTypeFullscreenSearch {
		if err := ash.TriggerLauncherStateChange(ctx, tconn, ash.AccelShiftSearch); err != nil {
			return errors.Wrap(err, "failed to switch to fullscreen")
		}
		if err := ash.WaitForLauncherState(ctx, tconn, ash.FullscreenSearch); err != nil {
			return errors.Wrap(err, "failed to switch the state to 'FullscreenSearch'")
		}
	}

	// Close
	if err := ash.TriggerLauncherStateChange(ctx, tconn, ash.AccelSearch); err != nil {
		return errors.Wrap(err, "failed to close launcher")
	}
	if err := ash.WaitForLauncherState(ctx, tconn, ash.Closed); err != nil {
		return errors.Wrap(err, "failed to switch the state to 'Closed'")
	}

	return nil
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

	// Non Home Launcher is only avaiable in clamshell mode.
	if originalTabletMode {
		ash.SetTabletModeEnabled(ctx, tconn, false)
		defer ash.SetTabletModeEnabled(ctx, tconn, originalTabletMode)
	}

	// TODO(oshima|mukai): run animation once to force creating a
	// launcher widget once we have a utility to initialize the
	// prevHists with current data. (crbug.com/1024071)

	pv := perf.NewValues()
	currentWindows := 0
	prevHists := map[string]*metrics.Histogram{}
	// Run the launcher open/close flow for various situations.
	// - change the number of browser windows, 0 or 2.
	// - peeking->close, peeking->half, peeking->half->fullscreen->close, fullscreen->close.
	for _, windows := range []int{0, 2} {
		conns, err := ash.CreateWindows(ctx, cr, ui.PerftestURL, windows-currentWindows)
		if err != nil {
			s.Fatal("Failed to create browser windows: ", err)
		}
		if err := conns.Close(); err != nil {
			s.Error("Failed to close the connection to chrome")
		}
		currentWindows = windows
		// The best effort to stabilize CPU usage. This may or
		// may not be satisfied in time.
		if err := cpu.WaitUntilIdle(ctx); err != nil {
			s.Error("Failed to wait for system UI to be stabilized: ", err)
		}

		for _, at := range []launcherAnimationType{animationTypePeeking, animationTypeHalf, animationTypeFullscreenSearch, animationTypeFullscreenAllApps} {
			// Wait for 1 seconds to stabilize the result. Note that this doesn't
			// have to be cpu.WaitUntilIdle(). It may wait too much.
			// TODO(mukai): find the way to wait more properly on the idleness of Ash.
			// https://crbug.com/1001314.
			if err := testing.Sleep(ctx, 1*time.Second); err != nil {
				s.Fatal("Failed to wait: ", err)
			}

			if err := runLauncherAnimation(ctx, tconn, kb, at); err != nil {
				s.Fatal("Fail to run launcher animation: ", err)
			}

			var suffix string
			switch at {
			case animationTypePeeking:
				suffix = "Peeking.ClamshellMode"
			case animationTypeFullscreenAllApps:
				suffix = "FullscreenAllApps.ClamshellMode"
			case animationTypeFullscreenSearch:
				suffix = "FullscreenSearch.ClamshellMode"
			case animationTypeHalf:
				suffix = "Half.ClamshellMode"
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
	}
	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
