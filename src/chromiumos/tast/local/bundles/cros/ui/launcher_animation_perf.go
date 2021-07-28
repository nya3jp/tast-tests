// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/perfutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/lacros"
	"chromiumos/tast/local/power"
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
			"cros-system-ui-eng@google.com",
		},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Timeout:      3 * time.Minute,
		Params: []testing.Param{{
			Val:     lacros.ChromeTypeChromeOS,
			Fixture: "chromeLoggedInWith100FakeApps",
		}, {
			Name:    "skia_renderer",
			Val:     lacros.ChromeTypeChromeOS,
			Fixture: "chromeLoggedInWith100FakeAppsSkiaRenderer",
		}, {
			Name:              "lacros",
			Val:               lacros.ChromeTypeLacros,
			Fixture:           "lacrosStartedByDataWith100FakeApps",
			ExtraSoftwareDeps: []string{"lacros"},
		}},
		Data: []string{"animation.html", "animation.js"},
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
	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	f := s.FixtValue()
	cr, err := lacros.GetChrome(ctx, f)
	if err != nil {
		s.Fatal("Failed to initialize test: ", err)
	}

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

	// Run an http server to serve the test contents for accessing from the chrome browsers.
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()
	url := server.URL + "/animation.html"

	// TODO(oshima|mukai): run animation once to force creating a
	// launcher widget once we have a utility to initialize the
	// prevHists with current data. (crbug.com/1024071)

	runner := perfutil.NewRunner(cr)
	currentWindows := 0
	// Run the launcher open/close flow for various situations.
	// - change the number of browser windows, 0 or 2.
	// - peeking->close, peeking->half, peeking->half->fullscreen->close, fullscreen->close.
	for _, windows := range []int{0, 2} {
		func() {
			_, l, cs, err := lacros.Setup(ctx, f, s.Param().(lacros.ChromeType))
			if err != nil {
				s.Fatal("Failed to setup lacrostest: ", err)
			}
			defer lacros.CloseLacrosChrome(ctx, l)

			if err := ash.CreateWindows(ctx, tconn, cs, url, windows-currentWindows); err != nil {
				s.Fatal("Failed to create browser windows: ", err)
			}
			// Maximize all windows to ensure a consistent state.
			if err := ash.ForEachWindow(ctx, tconn, func(w *ash.Window) error {
				return ash.SetWindowStateAndWait(ctx, tconn, w.ID, ash.WindowStateMaximized)
			}); err != nil {
				s.Fatal("Failed to maximize windows: ", err)
			}

			if s.Param().(lacros.ChromeType) == lacros.ChromeTypeLacros {
				if err := lacros.CloseAboutBlank(ctx, tconn, l.Devsess, 1); err != nil {
					s.Fatal("Failed to close about:blank: ", err)
				}
			}
			currentWindows = windows

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
				histograms := []string{
					"Apps.StateTransition.AnimationSmoothness." + suffix,
					"Apps.StateTransition.AnimationSmoothness.Close.ClamshellMode",
				}

				runner.RunMultiple(ctx, s, fmt.Sprintf("%s.%dwindows", suffix, currentWindows), perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
					return runLauncherAnimation(ctx, tconn, kb, at)
				}, histograms...),
					perfutil.StoreAll(perf.BiggerIsBetter, "percent", fmt.Sprintf("%dwindows", currentWindows)))
			}
		}()
	}
	if err := runner.Values().Save(ctx, s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
