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
	chromeUi "chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CardElementAnimationPerf,
		Desc:         "Measures animation smoothness of card elements and transition from peeking to half height",
		Contacts:     []string{"cowmoo@chromium.org", "xiaohuic@chromium.org"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Pre:          assistant.VerboseLoggingEnabled(),
	})
}

// CardElementAnimationPerf measures the animation performance of
// animating card elements in and out of the assistant frame. It also measures
// the performance of expanding the launcher from peeking to half height when
// a card is displayed.
func CardElementAnimationPerf(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	if err := assistant.EnableAndWaitForReady(ctx, tconn); err != nil {
		s.Fatal("Failed to enable Assistant: ", err)
	}
	defer func() {
		if err := assistant.Cleanup(ctx, s.HasError, cr, tconn); err != nil {
			s.Fatal("Failed to disable Assistant: ", err)
		}
	}()

	if err := assistant.SetBetterOnboardingEnabled(ctx, tconn, false); err != nil {
		s.Fatal("Failed to disable better onboarding: ", err)
	}

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
		)
		if err != nil {
			s.Fatal("Failed to collect histograms: ", err)
		}
		if err := assistantutils.ProcessHistogram(histograms, pv, nWindows); err != nil {
			s.Fatal("Failed to process histograms: ", err)
		}
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Fatal("Failed saving perf data: ", err)
	}
}

// runCardQueries performs two card queries in order to test the animate in
// and animate out behavior of the first card.
func runCardQueries(ctx context.Context, tconn *chrome.TestConn) error {
	for _, q := range []string{"Mount Everest", "Weather"} {
		if err := runCardQuery(ctx, tconn, q); err != nil {
			return err
		}
	}

	return nil
}

// runCardQuery is a helper function for running an Assistant query and waiting
// for a card result.
func runCardQuery(ctx context.Context, tconn *chrome.TestConn, query string) error {
	if _, err := assistant.SendTextQuery(ctx, tconn, query); err != nil {
		return errors.Wrapf(err, "could not send query: %s", query)
	}

	if _, err := chromeUi.StableFind(
		ctx,
		tconn,
		chromeUi.FindParams{ClassName: "AssistantCardElementView"},
		&testing.PollOptions{Timeout: 5 * time.Second},
	); err != nil {
		return errors.Wrapf(err, "query results not shown for query %s within timeout", query)
	}

	return nil
}
