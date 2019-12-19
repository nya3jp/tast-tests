// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/assistant"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AssistantEmbeddedUiAnimationPerf,
		Desc:         "Measures the smoothness of the Launcher-embedded Assistant UI animations",
		Contacts:     []string{"meilinw@chromium.org", "xiaohuic@chromium.org"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
		Timeout:      3 * time.Minute,
	})
}

// The only possible state change for app list is between peeking and closed when we open/close UI
// with the accelerator.
func toggleAssistantUiOpenAndClose(ctx context.Context, tconn *chrome.Conn) error {
	// closed->peeking.
	if err := assistant.ToggleUi(ctx, tconn); err != nil {
		return errors.Wrap(err, "Failed to open the embedded UI")
	}

	if err := ash.WaitForLauncherState(ctx, tconn, ash.Peeking); err != nil {
		return errors.Wrap(err, "failed to switch the state to 'Peeking'")
	}

	// peeking->closed.
	if err := assistant.ToggleUi(ctx, tconn); err != nil {
		return errors.Wrap(err, "Failed to close the embedded UI")
	}

	if err := ash.WaitForLauncherState(ctx, tconn, ash.Closed); err != nil {
		return errors.Wrap(err, "failed to switch the state to 'Closed'")
	}

	return nil
}

func AssistantEmbeddedUiAnimationPerf(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	if err := assistant.Enable(ctx, tconn); err != nil {
		s.Fatal("Failed to enable Assistant: ", err)
	}

	// We measure the open/close animation smoothness of the embedded container with 0, 1 or 2 background browser windows.
	const maxNumOfWindows = 2
	pv := perf.NewValues()
	for openedWindows := 0; openedWindows <= maxNumOfWindows; openedWindows++ {
		// We need to stabilize the CPU usage before the measurement happens. This may or
		// may not be satisfied in time.
		if err := cpu.WaitUntilIdle(ctx); err != nil {
			s.Error("Failed to wait for system UI to be stabilized: ", err)
		}

		histograms, err := metrics.Run(ctx, cr,
			func() error {
				if err := toggleAssistantUiOpenAndClose(ctx, tconn); err != nil {
					return err
				}
				return nil
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

		// Increases the number of browser windows opened in the background by 1 until we reach the target.
		if openedWindows < maxNumOfWindows {
			conns, err := ash.CreateWindows(ctx, cr, ui.PerftestURL, 1)
			if err != nil {
				s.Fatal("Failed to create a new browser window: ", err)
			}
			if err := conns.Close(); err != nil {
				s.Error("Failed to close the connection to chrome")
			}
		}
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
