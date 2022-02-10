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
	"chromiumos/tast/local/cpu"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LauncherToPeekingAnimationPerf,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Measures the animation smoothness of Assistant peeking mode while launcher is open",
		Contacts:     []string{"cowmoo@chromium.org", "xiaohuic@chromium.org"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Pre:          assistant.VerboseLoggingEnabled(),
	})
}

// LauncherToPeekingAnimationPerf measures the animation smoothness of showing
// and hiding the assistant while the launcher is open.
func LauncherToPeekingAnimationPerf(ctx context.Context, s *testing.State) {
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

	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to put into tablet mode: ", err)
	}
	defer cleanup(ctx)

	pv := perf.NewValues()
	for nWindows := 0; nWindows < 3; nWindows++ {
		if nWindows > 0 {
			if err := ash.CreateWindows(ctx, tconn, cr, ui.PerftestURL, 1); err != nil {
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
				return showAndHideAssistant(ctx, tconn, keyboard)
			},
			"Ash.Assistant.AnimationSmoothness.ResizeAssistantPageView",
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

func showAndHideAssistant(
	ctx context.Context,
	tconn *chrome.TestConn,
	keyboard *input.KeyboardEventWriter,
) error {
	if err := toggleLauncher(ctx, tconn, ash.Peeking); err != nil {
		return err
	}
	if err := assistant.ToggleUIWithHotkey(ctx, tconn, assistant.AccelAssistantKey); err != nil {
		return errors.Wrap(err, "failed to open the embedded UI")
	}
	if err := keyboard.Accel(ctx, "esc"); err != nil {
		return errors.Wrap(err, "failed to send escape key")
	}
	if err := toggleLauncher(ctx, tconn, ash.Closed); err != nil {
		return err
	}
	return nil
}
