// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	lacrostest "chromiumos/tast/local/bundles/cros/ui/lacros"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/lacros"
	lacroslauncher "chromiumos/tast/local/lacros/launcher"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: LauncherAnimationPerf,
		Desc: "Measures animation smoothness of lancher animations",
		Contacts: []string{
			"newcomer@chromium.org", "tbarzic@chromium.org", "cros-launcher-prod-notifications@google.com",
			"mukai@chromium.org", // original test author
		},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Timeout:      3 * time.Minute,
		Params: []testing.Param{{
			Val: lacros.ChromeTypeChromeOS,
			Pre: ash.LoggedInWith100DummyApps(),
		}, {
			Name:              "skia_renderer",
			Val:               lacros.ChromeTypeChromeOS,
			Pre:               ash.LoggedInWith100DummyAppsWithSkiaRenderer(),
			ExtraHardwareDeps: hwdep.D(hwdep.Model("nocturne", "krane")),
		}, {
			Name:      "lacros",
			Val:       lacros.ChromeTypeLacros,
			Pre:       lacroslauncher.StartedByDataWith100DummyApps(),
			ExtraData: []string{lacroslauncher.DataArtifact},
			// TODO(crbug.com/1082608): Use ExtraSoftwareDeps here instead.
			ExtraHardwareDeps: hwdep.D(hwdep.Model("eve")),
		}},
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

func runLauncherAnimation(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, at launcherAnimationType) error {
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
	cr, l, cs, err := lacrostest.Setup(ctx, s.PreValue(), s.Param().(lacros.ChromeType))
	if err != nil {
		s.Fatal("Failed to initialize test: ", err)
	}
	defer lacrostest.CloseLacrosChrome(ctx, l)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure in clamshell mode: ", err)
	}
	defer cleanup(ctx)

	// TODO(oshima|mukai): run animation once to force creating a
	// launcher widget once we have a utility to initialize the
	// prevHists with current data. (crbug.com/1024071)

	pv := perf.NewValues()
	currentWindows := 0
	// Run the launcher open/close flow for various situations.
	// - change the number of browser windows, 0 or 2.
	// - peeking->close, peeking->half, peeking->half->fullscreen->close, fullscreen->close.
	for _, windows := range []int{0, 2} {
		// Call Setup again in case the previous test closed all Lacros windows.
		cr, l, cs, err = lacrostest.Setup(ctx, s.PreValue(), s.Param().(lacros.ChromeType))
		if err != nil {
			s.Fatal("Failed to initialize test: ", err)
		}
		conns, err := ash.CreateWindows(ctx, tconn, cs, ui.PerftestURL, windows-currentWindows)
		if err != nil {
			s.Fatal("Failed to create browser windows: ", err)
		}
		// Maximize all windows to ensure a consistent state.
		if err := ash.ForEachWindow(ctx, tconn, func(w *ash.Window) error {
			_, err := ash.SetWindowState(ctx, tconn, w.ID, ash.WMEventMaximize)
			return err
		}); err != nil {
			s.Fatal("Failed to maximize windows: ", err)
		}

		if s.Param().(lacros.ChromeType) == lacros.ChromeTypeLacros {
			if err := lacros.CloseAboutBlank(ctx, l.Devsess); err != nil {
				s.Fatal("Failed to close about:blank: ", err)
			}
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

			histograms, err := metrics.RunAndWaitAll(ctx, tconn, time.Second, func() error {
				if err := runLauncherAnimation(ctx, tconn, kb, at); err != nil {
					return errors.Wrap(err, "fail to run launcher animation")
				}
				return nil
			},
				"Apps.StateTransition.AnimationSmoothness."+suffix,
				"Apps.StateTransition.AnimationSmoothness.Close.ClamshellMode")
			if err != nil {
				s.Fatal("Failed to run luancher animation or get histograms: ", err)
			}

			for _, h := range histograms {
				mean, err := h.Mean()
				if err != nil {
					s.Fatalf("Failed to get mean for histogram %s: %v", h.Name, err)
				}

				pv.Set(perf.Metric{
					Name:      fmt.Sprintf("%s.%dwindows", h.Name, currentWindows),
					Unit:      "percent",
					Direction: perf.BiggerIsBetter,
				}, mean)
			}
		}
	}
	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
