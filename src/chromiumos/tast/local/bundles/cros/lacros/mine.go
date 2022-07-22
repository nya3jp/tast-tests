// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/lacros/lacrosinfo"
	"chromiumos/tast/local/chrome/lacros/lacrosperf"
	"chromiumos/tast/local/cpu"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Mine,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Measure various Lacros startup latencies",
		Contacts: []string{
			"neis@google.com", // Test author
			"lacros-team@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Params: []testing.Param{{
			Name: "ash",
			Val:  browser.TypeAsh,
		}, {
			Name: "lacros",
			Val:  browser.TypeLacros,
		}},
	})
}

func Mine(ctx context.Context, s *testing.State) {
	bt := s.Param().(browser.Type)

	cr, err := browserfixt.NewChrome(ctx, bt, lacrosfixt.NewConfig(lacrosfixt.KeepAlive(false)), chrome.DisableFeatures("FirmwareUpdaterApp"))
	if err != nil {
		s.Fatal("Failed to restart Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	cleanup, err := lacrosperf.SetupPerfTest(ctx, tconn, "lacros.Mine")
	if err != nil {
		s.Fatal("Failed to prepare environment: ", err)
	}
	defer func() {
		if err := cleanup(ctx); err != nil {
			s.Fatal("Failed to cleanup after preparing environment: ", err)
		}
	}()

	pv := perf.NewValues()
	const iterationCount = 10
	for i := 0; i < iterationCount; i++ {
		if bt == browser.TypeLacros {
			if err := ensureLacrosStopped(ctx, tconn); err != nil {
				s.Fatal("Failed to ensure Lacros is stopped: ", err)
			}
		}
		if err := cpu.WaitUntilStabilized(ctx, cpu.DefaultCoolDownConfig(cpu.CoolDownPreserveUI)); err != nil {
			s.Fatal("Failed to wait for stabilized cpu: ", err)
		}

		startTime := time.Now()
		if err := openBrowser(ctx, tconn, bt, kb); err != nil {
			s.Fatal("Failed to open browser: ", err)
		}
		elapsedTime := time.Since(startTime)

		if err := ash.CloseAllWindows(ctx, tconn); err != nil {
			s.Fatal("Failed to close windows: ", err)
		}

		s.Logf("Iteration %d: %v", i, elapsedTime)
		name := "browser_startup_latency_newtab"
		if i == 0 {
			name += "_cold"
		} else {
			name += "_warm"
		}
		pv.Append(perf.Metric{
			Name:      name,
			Variant:   string(bt),
			Unit:      "ms",
			Direction: perf.SmallerIsBetter,
			Multiple:  true,
		}, float64(elapsedTime.Milliseconds()))
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}

func ensureLacrosStopped(ctx context.Context, tconn *chrome.TestConn) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		lacrosInfo, err := lacrosinfo.Snapshot(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to obtain Lacros info")
		}
		if !lacrosInfo.Stopped {
			testing.ContextLog(ctx, "Lacros is not yet stopped, waiting a bit longer")
			return errors.New("Lacros is not stopped")
		}
		return nil
	}, nil); err != nil {
		return err
	}
	return nil
}

// NOTE: This is the function being timed -- it should have minimal overhead.
func openBrowser(ctx context.Context, tconn *chrome.TestConn, bt browser.Type, kb *input.KeyboardEventWriter) error {
	if err := kb.Accel(ctx, "Ctrl+t"); err != nil {
		return errors.Wrap(err, "failed to send Ctrl+t")
	}
	if err := ash.WaitForCondition(ctx, tconn, ash.BrowserTypeMatch(bt), &testing.PollOptions{Interval: 5 * time.Millisecond}); err != nil {
		return errors.Wrapf(err, "failed to wait for the %v window to open", bt)
	}
	return nil
}
