// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	uiperf "chromiumos/tast/local/bundles/cros/ui/perf"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/perfutil"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WindowCyclePerf,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Measures the animation smoothness of window cycle animations when alt + tabbing",
		Contacts:     []string{"yjliu@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      3 * time.Minute,
		Params: []testing.Param{{
			Val:               browser.TypeAsh,
			Fixture:           "chromeLoggedIn",
			ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		}, {
			Name:              "lacros",
			Val:               browser.TypeLacros,
			Fixture:           "lacros",
			ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
			ExtraSoftwareDeps: []string{"lacros"},
		}, {
			// Pilot test on "noibat" that has HDMI dongle installed.
			Name:              "noibat",
			Val:               browser.TypeAsh,
			Fixture:           "chromeLoggedIn",
			ExtraHardwareDeps: hwdep.D(hwdep.Model("noibat")),
		}},
	})
}

func WindowCyclePerf(ctx context.Context, s *testing.State) {
	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	cr, l, cs, err := lacros.Setup(ctx, s.FixtValue(), s.Param().(browser.Type))
	if err != nil {
		s.Fatal("Failed to initialize test: ", err)
	}
	defer lacros.CloseLacros(ctx, l)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure in clamshell mode: ", err)
	}
	defer cleanup(ctx)

	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	numExistingWindows := 0

	runner := perfutil.NewRunner(cr.Browser())
	// If these window number values are changed, make sure to check lacros blank pages are closed correctly.
	for i, numWindows := range []int{2, 8} {
		if err := ash.CreateWindows(ctx, tconn, cs, ui.PerftestURL, numWindows-numExistingWindows); err != nil {
			s.Fatal("Failed to open browser windows: ", err)
		}

		// This must be done after ash.CreateWindows to avoid terminating lacros-chrome.
		if i == 0 && s.Param().(browser.Type) == browser.TypeLacros {
			if err := l.Browser().CloseWithURL(ctx, chrome.NewTabURL); err != nil {
				s.Fatal("Failed to close blank tab: ", err)
			}
		}

		// Maximize all windows to ensure a consistent state.
		if err := ash.ForEachWindow(ctx, tconn, func(w *ash.Window) error {
			return ash.SetWindowStateAndWait(ctx, tconn, w.ID, ash.WindowStateMaximized)
		}); err != nil {
			s.Fatal("Failed to maximize windows: ", err)
		}

		// TODO(crbug.com/1171056): Lacros may consume the Alt from Alt-tab after being maximized, without this sleep.
		if err := testing.Sleep(ctx, 1000*time.Millisecond); err != nil {
			s.Fatal("Failed to wait: ", err)
		}

		numExistingWindows = numWindows

		suffix := fmt.Sprintf("%dwindows", numWindows)
		runner.RunMultiple(ctx, suffix, uiperf.Run(s, perfutil.RunAndWaitAny(tconn, func(ctx context.Context) error {
			// Create a shorter context to ensure the time to release the alt-key.
			sctx, cancel := ctxutil.Shorten(ctx, 500*time.Millisecond)
			defer cancel()
			// first long press alt + tab to bring up the window cycle view
			if err := keyboard.AccelPress(sctx, "Alt"); err != nil {
				return errors.Wrap(err, "failed to press alt")
			}
			defer keyboard.AccelRelease(ctx, "Alt")
			if err := testing.Sleep(sctx, 500*time.Millisecond); err != nil {
				return errors.Wrap(err, "failed to wait")
			}
			if err := keyboard.Accel(sctx, "Tab"); err != nil {
				return errors.Wrap(err, "failed to type tab")
			}

			for i := 0; i < numWindows*2; i++ {
				if err := keyboard.Accel(sctx, "Tab"); err != nil {
					return errors.Wrap(err, "failed to type tab")
				}
				if err := testing.Sleep(sctx, 200*time.Millisecond); err != nil {
					return errors.Wrap(err, "failed to wait")
				}
			}

			if err := testing.Sleep(sctx, time.Second); err != nil {
				return errors.Wrap(err, "failed to wait")
			}
			return nil
		},
			"Ash.WindowCycleView.AnimationSmoothness.Show",
			"Ash.WindowCycleView.AnimationSmoothness.Container")),
			func(ctx context.Context, pv *perfutil.Values, hists []*metrics.Histogram) error {
				for _, hist := range hists {
					mean, err := hist.Mean()
					if err != nil {
						continue
					}
					name := hist.Name + "." + suffix
					testing.ContextLog(ctx, name, " = ", mean)
					pv.Append(perf.Metric{
						Name:      name,
						Unit:      "percent",
						Direction: perf.BiggerIsBetter,
					}, mean)
				}
				return nil
			})
	}

	if err = runner.Values().Save(ctx, s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
