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

var uiPollOptions = testing.PollOptions{
	Timeout:  8 * time.Second,
	Interval: 500 * time.Millisecond,
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         SuggestionChipAnimationPerf,
		Desc:         "Measures the animation smoothness of Assistant suggestion chips",
		Contacts:     []string{"cowmoo@chromium.org", "xiaohuic@chromium.org"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Pre:          assistant.VerboseLoggingEnabled(),
	})
}

// SuggestionChipAnimationPerf measures the animation smoothness of showing
// and hiding assistant suggestion chips upon executing a query and clicking on
// suggestions.
func SuggestionChipAnimationPerf(ctx context.Context, s *testing.State) {
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

	if err := assistant.SetVoiceInteractionConsentValue(ctx, tconn, 1); err != nil {
		s.Fatal("Failed to set voice interaction consent value: ", err)
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

		histograms, err := metrics.RunAndWaitAll(
			ctx,
			tconn,
			time.Second,
			func(ctx context.Context) error {
				return runQueriesAndClickSuggestions(ctx, tconn)
			},
			"Ash.Assistant.AnimationSmoothness.SuggestionChip",
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

func selectAndClickSuggestion(ctx context.Context, tconn *chrome.TestConn) error {
	if err := chromeUi.StableFindAndClick(
		ctx,
		tconn,
		chromeUi.FindParams{ClassName: "SuggestionChipView"},
		&uiPollOptions,
	); err != nil {
		return errors.Wrap(err, "failed to find and click assistant suggestion chip")
	}

	return nil
}

func runQueryAndWaitForCard(ctx context.Context, tconn *chrome.TestConn, query string) error {
	if _, err := assistant.SendTextQuery(ctx, tconn, query); err != nil {
		return errors.Wrapf(err, "unable to run query %s", query)
	}

	if _, err := chromeUi.StableFind(
		ctx,
		tconn,
		chromeUi.FindParams{ClassName: "AssistantCardElementView"},
		&uiPollOptions,
	); err != nil {
		return errors.Wrap(err, "timed out waiting for assistant card")
	}

	return nil
}

func runQueriesAndClickSuggestions(ctx context.Context, tconn *chrome.TestConn) error {
	if err := assistant.ToggleUIWithHotkey(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to open the assistant UI")
	}

	if err := runQueryAndWaitForCard(ctx, tconn, "Mount Kilimanjaro"); err != nil {
		return err
	}

	if err := selectAndClickSuggestion(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to run query and wait")
	}

	if err := runQueryAndWaitForCard(ctx, tconn, "Mount Everest"); err != nil {
		return errors.Wrap(err, "failed to run query and wait")
	}

	if err := selectAndClickSuggestion(ctx, tconn); err != nil {
		return err
	}

	if err := assistant.ToggleUIWithHotkey(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to open the assistant UI")
	}

	return nil
}
