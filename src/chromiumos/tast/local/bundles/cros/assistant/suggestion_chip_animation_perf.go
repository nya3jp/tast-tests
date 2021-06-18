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
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/media/cpu"
	uiConstants "chromiumos/tast/local/ui"
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

	ui := uiauto.New(tconn)

	for nWindows := 0; nWindows < 3; nWindows++ {
		if nWindows > 0 {
			if err := ash.CreateWindows(ctx, tconn, cr, uiConstants.PerftestURL, 1); err != nil {
				s.Fatal("Failed to create a new browser window: ", err)
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
				return runQueriesAndClickSuggestions(ctx, tconn, ui)
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

func runQueryAndClickSuggestion(ctx context.Context, tconn *chrome.TestConn, ui *uiauto.Context, query string) error {
	if _, err := assistant.SendTextQuery(ctx, tconn, query); err != nil {
		return errors.Wrapf(err, "unable to run query %s", query)
	}

	return uiauto.Combine("run query and click suggestion chip",
		ui.WithPollOpts(uiPollOptions).WaitUntilExists(nodewith.ClassName("AssistantCardElementView")),
		ui.WithPollOpts(uiPollOptions).LeftClick(nodewith.ClassName("SuggestionChipView").First()),
	)(ctx)
}

func runQueriesAndClickSuggestions(ctx context.Context, tconn *chrome.TestConn, ui *uiauto.Context) error {
	if err := assistant.ToggleUIWithHotkey(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to open the assistant UI")
	}

	if err := runQueryAndClickSuggestion(ctx, tconn, ui, "Mount Kilimanjaro"); err != nil {
		return err
	}

	if err := runQueryAndClickSuggestion(ctx, tconn, ui, "Mount Everest"); err != nil {
		return errors.Wrap(err, "failed to run query and wait")
	}

	if err := assistant.ToggleUIWithHotkey(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to open the assistant UI")
	}

	return nil
}
