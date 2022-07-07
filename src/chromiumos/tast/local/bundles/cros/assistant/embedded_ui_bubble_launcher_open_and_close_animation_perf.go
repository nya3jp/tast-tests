// Copyright 2022 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/local/bundles/cros/assistant/assistantutils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
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
		Timeout:      3 * time.Minute,
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

// toggleEmbeddedUI opens/closes the Launcher-embedded Assistant UI via hotkey.
// show indicates whether the hotkey is expected to show or hide the UI
func toggleEmbeddedUI(ctx context.Context, tconn *chrome.TestConn, accel assistant.Accelerator, show bool) error {
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

func EmbeddedUIBubbleLauncherOpenAndCloseAnimationPerf(ctx context.Context, s *testing.State) {
	accel := s.Param().(assistant.Accelerator)

	fixtData := s.FixtValue().(*assistant.FixtData)
	cr := fixtData.Chrome

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
		assistantutils.RecordAnimationPerformance(ctx, tconn, pv,
			[]string{"Apps.ClamshellLauncher.AnimationSmoothness.Open"},
			func(ctx context.Context) error {
				return toggleEmbeddedUI(ctx, tconn, accel, true)
			},
			func(h *metrics.Histogram) string {
				return fmt.Sprintf("%s.%dwindows", h.Name, openedWindows)
			},
		)

		assistantutils.RecordAnimationPerformance(ctx, tconn, pv,
			[]string{"Apps.ClamshellLauncher.AnimationSmoothness.Close"},
			func(ctx context.Context) error {
				return toggleEmbeddedUI(ctx, tconn, accel, false)
			},
			func(h *metrics.Histogram) string {
				return fmt.Sprintf("%s.%dwindows", h.Name, openedWindows)
			},
		)

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
