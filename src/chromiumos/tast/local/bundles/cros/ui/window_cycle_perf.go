// Copyright 2019 The Chromium OS Authors. All rights reserved.
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
	lacrostest "chromiumos/tast/local/bundles/cros/ui/lacros"
	"chromiumos/tast/local/bundles/cros/ui/perfutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/lacros"
	"chromiumos/tast/local/lacros/launcher"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WindowCyclePerf,
		Desc:         "Measures the animation smoothness of window cycle animations when alt + tabbing",
		Contacts:     []string{"yjliu@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Timeout:      3 * time.Minute,
		Params: []testing.Param{{
			Val: lacros.ChromeTypeChromeOS,
			Pre: chrome.LoggedIn(),
		}, {
			Name:      "lacros",
			Val:       lacros.ChromeTypeLacros,
			Pre:       launcher.StartedByData(),
			ExtraData: []string{launcher.DataArtifact},
			// TODO(crbug.com/1082608): Use ExtraSoftwareDeps here instead.
			ExtraHardwareDeps: hwdep.D(hwdep.Model("eve")),
		}},
	})
}

func WindowCyclePerf(ctx context.Context, s *testing.State) {
	cr, l, cs, err := lacrostest.Setup(ctx, s.PreValue(), s.Param().(lacros.ChromeType))
	if err != nil {
		s.Fatal("Failed to initialize test: ", err)
	}
	defer lacrostest.CloseLacrosChrome(ctx, l)

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

	runner := perfutil.NewRunner(cr)
	for _, numWindows := range []int{2, 8} {
		conns, err := ash.CreateWindows(ctx, tconn, cs, ui.PerftestURL, numWindows-numExistingWindows)
		if err != nil {
			s.Fatal("Failed to open browser windows: ", err)
		}
		conns.Close()

		if s.Param().(lacros.ChromeType) == lacros.ChromeTypeLacros {
			if err := lacros.CloseAboutBlank(ctx, l.Devsess); err != nil {
				s.Fatal("Failed to close about:blank: ", err)
			}
		}

		numExistingWindows = numWindows

		if err = cpu.WaitUntilIdle(ctx); err != nil {
			s.Fatal("Failed waiting for CPU to become idle: ", err)
		}

		suffix := fmt.Sprintf("%dwindows", numWindows)
		runner.RunMultiple(ctx, s, suffix, perfutil.RunAndWaitAny(tconn, func() error {
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
			"Ash.WindowCycleView.AnimationSmoothness.Container"),
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

	if err = runner.Values().Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
