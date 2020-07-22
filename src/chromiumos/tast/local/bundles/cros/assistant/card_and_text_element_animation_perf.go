// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package assistant

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/assistant"
	"chromiumos/tast/local/bundles/cros/assistant/assistantutils"
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
		Func:         CardAndTextElementAnimationPerf,
		Desc:         "Measures animation smoothness of card elements and transition from peeking to half height",
		Contacts:     []string{"cowmoo@chromium.org", "xiaohuic@chromium.org"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Pre:          assistant.VerboseLoggingEnabled(),
	})
}

// CardAndTextElementAnimationPerf measures the animation performance of
// animating card elements in and out of the assistant frame. It also measures
// the performance of expanding the launcher from peeking to half height when
// a card is displayed.
func CardAndTextElementAnimationPerf(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	if err := assistant.EnableAndWaitForReady(ctx, tconn); err != nil {
		s.Fatal("Failed to enable Assistant: ", err)
	}
	defer assistant.Cleanup(ctx, s, cr, tconn)

	pv := perf.NewValues()
	for nWindows := 0; nWindows < 3; nWindows++ {
		if nWindows > 0 {
			conns, err := ash.CreateWindows(ctx, tconn, cr, ui.PerftestURL, 1)

			if err != nil {
				s.Fatal("Failed to create a new browser window: ", err)
			}

			if err := conns.Close(); err != nil {
				s.Error("Failed to close the connection to a browser window: ", err)
			}
		}

		if err := cpu.WaitUntilIdle(ctx); err != nil {
			s.Error("Failed to wait for system cpu idle: ", err)
		}

		if err := assistant.ToggleUIWithHotkey(ctx, tconn); err != nil {
			s.Fatal("Failed opening assistant: ", err)
		}

		histograms, err := metrics.RunAndWaitAll(
			ctx,
			tconn,
			time.Second,
			func(ctx context.Context) error {
				return runCardQueries(ctx, tconn)
			},
			// Card element opacity fade in / out.
			"Ash.Assistant.AnimationSmoothness.CardElement",
			// Launcher expanding to half when the first card element appears.
			"Apps.StateTransition.AnimationSmoothness.Half.ClamshellMode",
		)
		if err != nil {
			s.Fatal("Failed to collect histograms: ", err)
		}
		assistantutils.ProcessHistogram(histograms, pv, nWindows)
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Fatal("Failed saving perf data: ", err)
	}
}

// runCardQueries performs two card queries in order to test the animate in
// and animate out behavior of the first card.
func runCardQueries(ctx context.Context, tconn *chrome.TestConn) error {
	_, err := assistant.SendTextQuery(ctx, tconn, "Mount Everest")
	if err != nil {
		return errors.Wrap(err, "could not send query: \"Mount Everest\"")
	}
	_, err = assistant.SendTextQuery(ctx, tconn, "Weather")
	if err != nil {
		return errors.Wrap(err, "could not send query: \"Weather\"")
	}
	return nil
}
