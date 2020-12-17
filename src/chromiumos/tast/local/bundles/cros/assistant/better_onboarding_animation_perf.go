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
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BetterOnboardingAnimationPerf,
		Desc:         "Measures animation smoothness of opening assistant with Better Onboarding enabled",
		Contacts:     []string{"cowmoo@chromium.org", "xiaohuic@chromium.org"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Pre:          assistant.VerboseLoggingEnabled(),
	})
}

// BetterOnboardingAnimationPerf measures the performance of opening the Assistant UI
// for the first and second time in a session when Better Onboarding shows. It is believed
// that loading Better Onboarding assets from disk on first launch causes a noticeable
// animation smoothness performance hit.
func BetterOnboardingAnimationPerf(ctx context.Context, s *testing.State) {
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

	if err := assistant.SetBetterOnboardingEnabled(ctx, tconn, true); err != nil {
		s.Fatal("Failed to enable better onboarding: ", err)
	}

	cleanupTabletMode, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to enable tablet mode: ", err)
	}
	defer cleanupTabletMode(ctx)

	pv := perf.NewValues()

	// Open and close Assistant UI for the first time in the session.
	if err := gatherMetrics(ctx, tconn, pv, "first_run"); err != nil {
		s.Fatal("Failed to gather first run metrics: ", err)
	}

	// Open and close Assistant UI again for comparison.
	if err := gatherMetrics(ctx, tconn, pv, "second_run"); err != nil {
		s.Fatal("Failed to gather second run metrics: ", err)
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}

func gatherMetrics(
	ctx context.Context,
	tconn *chrome.TestConn,
	pv *perf.Values,
	postfix string,
) error {

	// Open the Assistant UI and gather metrics.
	histograms, err := metrics.RunAndWaitAll(
		ctx,
		tconn,
		time.Second,
		func(ctx context.Context) error {
			if err := assistant.ToggleUIWithHotkey(ctx, tconn); err != nil {
				return errors.Wrap(err, "failed to open Assistant UI")
			}
			return nil
		},
		"Ash.Assistant.AnimationSmoothness.ResizeAssistantPageView",
		"Apps.StateTransition.AnimationSmoothness",
	)
	if err != nil {
		return errors.Wrap(err, "failed to collect histogram")
	}

	for _, h := range histograms {
		mean, err := h.Mean()
		if err != nil {
			return errors.Wrapf(err, "failed to get mean for histogram %s", h.Name)
		}

		pv.Set(
			perf.Metric{
				Name:      fmt.Sprintf("%s.%s", h.Name, postfix),
				Unit:      "percent",
				Direction: perf.BiggerIsBetter,
			},
			mean,
		)
	}

	// Close the Assistant UI.
	if _, err := metrics.RunAndWaitAll(
		ctx,
		tconn,
		time.Second,
		func(ctx context.Context) error {
			return assistant.ToggleUIWithHotkey(ctx, tconn)
		},
		"Apps.StateTransition.AnimationSmoothness",
	); err != nil {
		return errors.Wrap(err, "failed to close the assistant UI")
	}

	return nil
}
