// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package assistant

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/assistant"
	"chromiumos/tast/local/bundles/cros/assistant/assistantutils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/cpu"
	uiconsts "chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
)

var uiPollOptions = testing.PollOptions{
	Timeout:  8 * time.Second,
	Interval: 500 * time.Millisecond,
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         SuggestionChipAnimationPerf,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Measures the animation smoothness of Assistant suggestion chips",
		Contacts:     []string{"cowmoo@chromium.org", "xiaohuic@chromium.org"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Fixture:      "assistantPerf",
	})
}

// SuggestionChipAnimationPerf measures the animation smoothness of showing
// and hiding assistant suggestion chips upon executing a query and clicking on
// suggestions.
func SuggestionChipAnimationPerf(ctx context.Context, s *testing.State) {
	fixtData := s.FixtValue().(*assistant.FixtData)
	cr := fixtData.Chrome
	accel, err := assistant.ResolveAssistantHotkey(s.Features(""))
	if err != nil {
		s.Fatal("Failed to resolve assistant hotkey: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	if err := assistant.SetBetterOnboardingEnabled(ctx, tconn, false); err != nil {
		s.Fatal("Failed to disable better onboarding: ", err)
	}

	if err := assistant.SetVoiceInteractionConsentValue(ctx, tconn, 1); err != nil {
		s.Fatal("Failed to set voice interaction consent value: ", err)
	}

	pv := perf.NewValues()

	for nWindows := 0; nWindows < 3; nWindows++ {
		if nWindows > 0 {
			if err := ash.CreateWindows(ctx, tconn, cr, uiconsts.PerftestURL, 1); err != nil {
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
				return runQueriesAndClickSuggestions(ctx, tconn, accel)
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

func runQueryAndClickSuggestion(tconn *chrome.TestConn, query string) action.Action {
	return func(ctx context.Context) error {
		if _, err := assistant.SendTextQuery(ctx, tconn, query); err != nil {
			return errors.Wrapf(err, "unable to run query %s", query)
		}
		ui := uiauto.New(tconn).WithPollOpts(uiPollOptions)
		return uiauto.Combine("run query and click suggestion chip",
			ui.WaitUntilExists(nodewith.ClassName("AssistantCardElementView")),
			ui.LeftClick(nodewith.ClassName("SuggestionChipView").First()),
		)(ctx)
	}
}

func toggleAssistantWithHotkey(tconn *chrome.TestConn, accel assistant.Accelerator) action.Action {
	return func(ctx context.Context) error {
		return assistant.ToggleUIWithHotkey(ctx, tconn, accel)
	}
}

func runQueriesAndClickSuggestions(ctx context.Context, tconn *chrome.TestConn, accel assistant.Accelerator) error {
	return uiauto.Combine("toggle ui and run queries and click suggestion chip",
		toggleAssistantWithHotkey(tconn, accel),
		runQueryAndClickSuggestion(tconn, "Mount Kilimanjaro"),
		runQueryAndClickSuggestion(tconn, "Mount Everest"),
		toggleAssistantWithHotkey(tconn, accel),
	)(ctx)
}
