// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package assistant

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/assistant"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         EmbeddedUIOpenAndCloseAnimationPerf,
		Desc:         "Measures the smoothness of the embedded UI open and close animation",
		Contacts:     []string{"meilinw@chromium.org", "xiaohuic@chromium.org"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Pre:          chrome.LoggedIn(),
		Timeout:      3 * time.Minute,
	})
}

// openAndCloseEmbeddedUI opens/closes the Launcher-embedded Assistant UI via hotkey.
// The only possible state change of Launcher it can trigger is between peeking and closed.
func openAndCloseEmbeddedUI(ctx context.Context, tconn *chrome.TestConn) error {
	// Closed->Peeking.
	if err := assistant.ToggleUIWithHotkey(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to open the embedded UI")
	}

	if err := ash.WaitForLauncherState(ctx, tconn, ash.Peeking); err != nil {
		return errors.Wrap(err, "failed to switch the state to 'Peeking'")
	}

	// Peeking->Closed.
	if err := assistant.ToggleUIWithHotkey(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to close the embedded UI")
	}

	if err := ash.WaitForLauncherState(ctx, tconn, ash.Closed); err != nil {
		return errors.Wrap(err, "failed to switch the state to 'Closed'")
	}

	return nil
}

func EmbeddedUIOpenAndCloseAnimationPerf(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	if err := assistant.Enable(ctx, tconn); err != nil {
		s.Fatal("Failed to enable Assistant: ", err)
	}
	defer assistant.Cleanup(ctx, s, cr, tconn)

	// Ensures the test only run under the clamshell mode.
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure in clamshell mode: ", err)
	}
	defer cleanup(ctx)

	// Enables the "Related Info" setting for Assistant.
	// We explicitly enable this setting here because it controlled the root cause
	// of the open animation jankiness in b/145218971.
	if err := assistant.SetContextEnabled(ctx, tconn, true); err != nil {
		s.Fatal("Failed to enable context for Assistant: ", err)
	}
	defer func() {
		// Reset the pref value at the end for clean-up.
		if err := assistant.SetContextEnabled(ctx, tconn, false); err != nil {
			s.Fatal("Failed to disable context for Assistant: ", err)
		}
	}()

	// We measure the open/close animation smoothness of the embedded UI with 0, 1 or 2
	// browser windows in the background.
	const maxNumOfWindows = 2
	pv := perf.NewValues()
	for openedWindows := 0; openedWindows <= maxNumOfWindows; openedWindows++ {
		// We need to stabilize the CPU usage before the measurement happens. This may or
		// may not be satisfied in time.
		if err := cpu.WaitUntilIdle(ctx); err != nil {
			s.Error("Failed to wait for system UI to be stabilized: ", err)
		}

		histograms, err := metrics.RunAndWaitAll(ctx, tconn, time.Second,
			func() error {
				return openAndCloseEmbeddedUI(ctx, tconn)
			},
			"Apps.StateTransition.AnimationSmoothness.Peeking.ClamshellMode",
			"Apps.StateTransition.AnimationSmoothness.Close.ClamshellMode")

		if err != nil {
			s.Fatal("Failed to run embedded UI animation or get histograms: ", err)
		}

		// Collects the histogram results.
		for _, h := range histograms {
			mean, err := h.Mean()
			if err != nil {
				s.Fatalf("Failed to get mean for histogram %s: %v", h.Name, err)
			}

			pv.Set(perf.Metric{
				Name:      fmt.Sprintf("%s.%dwindows", h.Name, openedWindows),
				Unit:      "percent",
				Direction: perf.BiggerIsBetter,
			}, mean)
		}

		// Increases the number of browser windows opened in the background by 1
		// until we reach the target.
		if openedWindows < maxNumOfWindows {
			conns, err := ash.CreateWindows(ctx, tconn, cr, ui.PerftestURL, 1)
			if err != nil {
				s.Fatal("Failed to create a new browser window: ", err)
			}
			if err := conns.Close(); err != nil {
				s.Error("Failed to close the connection to a browser window")
			}
		}
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
