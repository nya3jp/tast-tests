// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package assistant

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/local/assistant"
	"chromiumos/tast/local/bundles/cros/assistant/assistantutils"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         EmbeddedUIBubbleLauncherOpenAndCloseAnimationPerf,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Measures the smoothness of the bubble launcher embedded UI open and close animation",
		Contacts:     []string{"xiaohuic@chromium.org", "assistive-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Fixture:      "assistantClamshellPerf",
		// Due to b/238758287, first close animation can always take 2 mins.
		Timeout: 6 * time.Minute,
	})
}

func EmbeddedUIBubbleLauncherOpenAndCloseAnimationPerf(ctx context.Context, s *testing.State) {
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

	// If a DUT switches from Tablet mode to Clamshell mode, it can take a while until launcher gets settled down.
	if err := ash.WaitForLauncherState(ctx, tconn, ash.Closed); err != nil {
		s.Fatal("Failed to wait the launcher state Closed: ", err)
	}

	// Enables the "Related Info" setting for Assistant.
	// We explicitly enable this setting here because it controlled the root cause
	// of the open animation jankiness in b/145218971.
	if err := assistant.SetContextEnabled(ctx, tconn, true); err != nil {
		s.Fatal("Failed to enable context for Assistant: ", err)
	}
	defer func() {
		// Reset the pref value at the end for clean-up.
		if err := assistant.SetContextEnabled(ctx, tconn, false); err != nil {
			s.Fatal("Failed to disable context for Assistant: ", err)
		}
	}()

	if err := assistant.SetBetterOnboardingEnabled(ctx, tconn, false); err != nil {
		s.Fatal("Failed to disable onboarding for Assistant: ", err)
	}

	// We measure the open/close animation smoothness of the embedded UI with 0, 1 or 2
	// browser windows in the background.
	const maxNumOfWindows = 2
	pv := perf.NewValues()
	for openedWindows := 0; openedWindows <= maxNumOfWindows; openedWindows++ {
		if err := assistantutils.RecordAnimationPerformance(ctx, s, tconn, pv,
			[]string{"Apps.ClamshellLauncher.AnimationSmoothness.Open"},
			func(ctx context.Context) error {
				return assistant.ToggleUIWithHotkey(ctx, tconn, accel)
			},
			func(h *metrics.Histogram) string {
				return fmt.Sprintf("%s.%dwindows", h.Name, openedWindows)
			},
		); err != nil {
			s.Fatal("Failed to run performance test of opening embedded UI: ", err)
		}

		// TODO(b/238758287): Because of this issue, this always waits 2 mins for cpu idle time
		// for the first run (at least) on betty-vm.
		if err := assistantutils.RecordAnimationPerformance(ctx, s, tconn, pv,
			[]string{"Apps.ClamshellLauncher.AnimationSmoothness.Close"},
			func(ctx context.Context) error {
				return assistant.ToggleUIWithHotkey(ctx, tconn, accel)
			},
			func(h *metrics.Histogram) string {
				return fmt.Sprintf("%s.%dwindows", h.Name, openedWindows)
			},
		); err != nil {
			s.Fatal("Failed to run performance test of closing embedded UI: ", err)
		}

		// Increases the number of browser windows opened in the background by 1
		// until we reach the target.
		if openedWindows < maxNumOfWindows {
			if err := ash.CreateWindows(ctx, tconn, cr, ui.PerftestURL, 1); err != nil {
				s.Fatal("Failed to create a new browser window: ", err)
			}
		}
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
