// Copyright 2022 The ChromiumOS Authors
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
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BetterOnboardingBubbleLauncherAnimationPerf,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Measures animation smoothness of opening assistant in bubble launcher with Better Onboarding enabled",
		Contacts:     []string{"cowmoo@chromium.org", "xiaohuic@chromium.org", "assistive-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Fixture:      "assistantClamshellPerf",
	})
}

// BetterOnboardingBubbleLauncherAnimationPerf measures the performance of opening the Assistant UI
// for the first and second time in a session when Better Onboarding shows. It is believed
// that loading Better Onboarding assets from disk on first launch causes a noticeable
// animation smoothness performance hit.
func BetterOnboardingBubbleLauncherAnimationPerf(ctx context.Context, s *testing.State) {
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

	if err := assistant.SetBetterOnboardingEnabled(ctx, tconn, true); err != nil {
		s.Fatal("Failed to enable better onboarding: ", err)
	}
	defer func() {
		// Send a text query to clear "stickiness" of better onboarding for this
		// session. This prevents other Assistant tests from hitting better
		// onboarding if they run after this one.
		if _, err := assistant.SendTextQuery(ctx, tconn, "test query"); err != nil {
			s.Error("Failed to run text query to clear better onboarding: ", err)
		}
		if err := assistant.SetBetterOnboardingEnabled(ctx, tconn, false); err != nil {
			s.Error("Failed to disable better onboarding: ", err)
		}
	}()

	pv := perf.NewValues()

	// Open Assistant UI for the first time in the session.
	if err := recordBubbleLauncherAssistantToggleSmoothness(ctx, tconn, pv, "first_run", accel, true); err != nil {
		s.Fatal("Failed to gather first run open metrics: ", err)
	}

	// Close  Assistant UI for the first time in the session.
	if err := recordBubbleLauncherAssistantToggleSmoothness(ctx, tconn, pv, "first_run", accel, false); err != nil {
		s.Fatal("Failed to gather first run close metrics: ", err)
	}

	// Open and close Assistant UI again for comparison.
	if err := recordBubbleLauncherAssistantToggleSmoothness(ctx, tconn, pv, "second_run", accel, true); err != nil {
		s.Fatal("Failed to gather second run open metrics: ", err)
	}
	if err := recordBubbleLauncherAssistantToggleSmoothness(ctx, tconn, pv, "second_run", accel, false); err != nil {
		s.Fatal("Failed to gather second run close metrics: ", err)
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}

func recordBubbleLauncherAssistantToggleSmoothness(ctx context.Context, tconn *chrome.TestConn, pv *perf.Values, postfix string, accel assistant.Accelerator, show bool) error {
	var targetHistogram string
	if show {
		targetHistogram = "Apps.ClamshellLauncher.AnimationSmoothness.Open"
	} else {
		targetHistogram = "Apps.ClamshellLauncher.AnimationSmoothness.Close"
	}

	// Open/Close the Assistant UI and gather metrics.
	histograms, err := metrics.RunAndWaitAll(
		ctx,
		tconn,
		3*time.Second,
		func(ctx context.Context) error {
			return toggleBubbleLauncherAssistantUI(ctx, tconn, accel, show)
		},
		targetHistogram,
	)
	if err != nil {
		return errors.Wrap(err, "failed to collect histogram")
	}

	for _, h := range histograms {
		mean, err := h.Mean()

		if err != nil {
			return errors.Wrapf(err, "failed to get mean for histogram %s", h.Name)
		}

		pv.Set(perf.Metric{
			Name:      fmt.Sprintf("%s.%s", h.Name, postfix),
			Unit:      "percent",
			Direction: perf.BiggerIsBetter,
		}, mean)
	}

	return nil
}

// toggleBubbleLauncherAssistantUI toggles the Assistant UI via hotkey. show indicates whether the launcher and
// assistant UI is expected to be shown or hidden.
func toggleBubbleLauncherAssistantUI(ctx context.Context, tconn *chrome.TestConn, accel assistant.Accelerator, show bool) error {
	if err := assistant.ToggleUIWithHotkey(ctx, tconn, accel); err != nil {
		return errors.Wrap(err, "failed to open the embedded UI")
	}

	ui := uiauto.New(tconn)
	bubble := nodewith.ClassName(ash.AppListBubbleClassName)
	if show {
		if err := ui.WaitUntilExists(bubble)(ctx); err != nil {
			return errors.Wrap(err, "could not open launcher bubble by pressing assistant hotkey")
		}
	} else {
		if err := ui.WaitUntilGone(bubble)(ctx); err != nil {
			return errors.Wrap(err, "could not close launcher bubble by pressing assistant hotkey")
		}
	}

	return nil
}
