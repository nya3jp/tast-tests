// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package assistant

import (
	"context"
	"fmt"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/assistant"
	"chromiumos/tast/local/bundles/cros/assistant/assistantutils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/cpu"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LauncherToPeekingAnimationPerf,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Measures the animation smoothness of Assistant peeking mode while launcher is open",
		Contacts:     []string{"cowmoo@chromium.org", "xiaohuic@chromium.org"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Fixture:      "assistantClamshellWithLegacyLauncherPerf",
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

// LauncherToPeekingAnimationPerf measures the animation smoothness of showing
// and hiding the assistant while the launcher is open.
func LauncherToPeekingAnimationPerf(ctx context.Context, s *testing.State) {
	const histogramResizeAssistantPageView = "Ash.Assistant.AnimationSmoothness.ResizeAssistantPageView"

	accel := s.Param().(assistant.Accelerator)

	fixtData := s.FixtValue().(*assistant.FixtData)
	cr := fixtData.Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	if err := assistant.SetBetterOnboardingEnabled(ctx, tconn, false); err != nil {
		s.Fatal("Failed to disable better onboarding: ", err)
	}

	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	// If a DUT switches from Tablet mode to Clamshell mode, it can take a while until launcher gets settled down.
	if err := ash.WaitForLauncherState(ctx, tconn, ash.Closed); err != nil {
		s.Fatal("Failed to wait the launcher state Closed: ", err)
	}

	pv := perf.NewValues()
	for nWindows := 0; nWindows < 3; nWindows++ {
		if nWindows > 0 {
			if err := ash.CreateWindows(ctx, tconn, cr, ui.PerftestURL, 1); err != nil {
				s.Fatal("Failed to create a new browser window: ", err)
			}
		}

		if err := toggleLauncher(ctx, tconn, ash.Peeking); err != nil {
			s.Fatal("Failed to toggle launcher to peeking state: ", err)
		}

		if err := cpu.WaitUntilIdle(ctx); err != nil {
			s.Error("Failed to wait for system cpu idle: ", err)
		}

		// Measure animation performance of showing Assistant UI
		assistantutils.RecordAnimationPerformance(ctx, tconn, pv,
			[]string{histogramResizeAssistantPageView},
			func(ctx context.Context) error {
				return assistant.ToggleUIWithHotkey(ctx, tconn, accel)
			},
			func(h *metrics.Histogram) string {
				return fmt.Sprintf("%s.Show.%dwindows", h.Name, nWindows)
			},
		)

		// Measure animation performance of hiding Assistant UI
		assistantutils.RecordAnimationPerformance(ctx, tconn, pv,
			[]string{histogramResizeAssistantPageView},
			func(ctx context.Context) error {
				return keyboard.Accel(ctx, "esc")
			},
			func(h *metrics.Histogram) string {
				return fmt.Sprintf("%s.Hide.%dwindows", h.Name, nWindows)
			},
		)

		if err := toggleLauncher(ctx, tconn, ash.Closed); err != nil {
			s.Fatal("Failed to toggle launcher to closed state: ", err)
		}
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Fatal("Failed saving perf data: ", err)
	}
}

func toggleLauncher(
	ctx context.Context,
	tconn *chrome.TestConn,
	expectedState ash.LauncherState,
) error {
	if err := ash.TriggerLauncherStateChange(ctx, tconn, ash.AccelSearch); err != nil {
		return errors.Wrap(err, "failed to open launcher")
	}
	if err := ash.WaitForLauncherState(ctx, tconn, expectedState); err != nil {
		return errors.Wrapf(err, "failed to wait for launcher state %s", expectedState)
	}
	return nil
}
