// Copyright 2021 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BetterOnboardingAnimationPerf,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Measures animation smoothness of opening assistant with Better Onboarding enabled",
		Contacts:     []string{"cowmoo@chromium.org", "xiaohuic@chromium.org"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Pre:          chrome.LoggedIn(),
		Params: []testing.Param{
			{
				Name:              "assistant_key",
				Val:               assistant.AccelAssistantKey,
				ExtraHardwareDeps: hwdep.D(hwdep.AssistantKey()),
			},
			{
				Name:              "search_plus_a",
				Val:               assistant.AccelSearchPlusA,
				ExtraHardwareDeps: hwdep.D(hwdep.NoAssistantKey()),
			},
		},
	})
}

// BetterOnboardingAnimationPerf measures the performance of opening the Assistant UI
// for the first and second time in a session when Better Onboarding shows. It is believed
// that loading Better Onboarding assets from disk on first launch causes a noticeable
// animation smoothness performance hit.
func BetterOnboardingAnimationPerf(ctx context.Context, s *testing.State) {
	accel := s.Param().(assistant.Accelerator)
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
			s.Error("Failed to disable Assistant: ", err)
		}
	}()

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

	const IsTabletMode = false
	cleanupTabletMode, err := ash.EnsureTabletModeEnabled(ctx, tconn, IsTabletMode)
	if err != nil {
		s.Fatal("Failed to put into Clamshell mode: ", err)
	}
	defer cleanupTabletMode(ctx)

	// If a DUT switches from Tablet mode to Clamshell mode, it can take a while until launcher gets settled down.
	if err := ash.WaitForLauncherState(ctx, tconn, ash.Closed); err != nil {
		s.Fatal("Failed to wait the launcher state Closed: ", err)
	}

	pv := perf.NewValues()

	// Open and close Assistant UI for the first time in the session.
	if err := recordAssistantToggleSmoothness(ctx, tconn, pv, "first_run", accel); err != nil {
		s.Fatal("Failed to gather first run metrics: ", err)
	}

	// Open and close Assistant UI again for comparison.
	if err := recordAssistantToggleSmoothness(ctx, tconn, pv, "second_run", accel); err != nil {
		s.Fatal("Failed to gather second run metrics: ", err)
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}

func recordAssistantToggleSmoothness(ctx context.Context, tconn *chrome.TestConn, pv *perf.Values, postfix string, accel assistant.Accelerator) error {
	// Open the Assistant UI and gather metrics.
	// TODO(b:178409604) add "Apps.StateTransition.AnimationSmoothness.Half.ClamshellMode" when it is recorded properly.
	histograms, err := metrics.RunAndWaitAll(
		ctx,
		tconn,
		3*time.Second,
		func(ctx context.Context) error {
			return toggleAssistantUI(ctx, tconn, accel)
		},
		"Ash.Assistant.AnimationSmoothness.ResizeAssistantPageView",
		"Apps.StateTransition.AnimationSmoothness.Close.ClamshellMode",
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

// toggleAssistantUI opens and then closes the Assistant UI via hotkey. Launcher
// opens to Half state, rather than Peeking, because Better Onboarding requires extra space.
func toggleAssistantUI(ctx context.Context, tconn *chrome.TestConn, accel assistant.Accelerator) error {
	// Closed->Peeking.
	if err := assistant.ToggleUIWithHotkey(ctx, tconn, accel); err != nil {
		return errors.Wrap(err, "failed to open the embedded UI")
	}

	if err := ash.WaitForLauncherState(ctx, tconn, ash.Half); err != nil {
		return errors.Wrap(err, "failed to switch the state to 'Half'")
	}

	// Peeking->Closed.
	if err := assistant.ToggleUIWithHotkey(ctx, tconn, accel); err != nil {
		return errors.Wrap(err, "failed to close the embedded UI")
	}

	if err := ash.WaitForLauncherState(ctx, tconn, ash.Closed); err != nil {
		return errors.Wrap(err, "failed to switch the state to 'Closed'")
	}

	return nil
}
