// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/ui/tabswitchcuj"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/cuj"
	"chromiumos/tast/local/cpu"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/ui/cujrecorder"
	"chromiumos/tast/local/wpr"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TabSwitchCUJ,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Measures the performance of tab-switching CUJ",
		// TODO(b/234063928): Remove crosbolt attributes when TabSwitchCUJ runs stably on suite cuj.
		Attr:         []string{"group:cuj"},
		Contacts:     []string{"yichenz@chromium.org", "chromeos-perfmetrics-eng@google.com"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{cujrecorder.SystemTraceConfigFile},
		Timeout:      22*time.Minute + cuj.CPUStablizationTimeout,
		Vars: []string{
			"mute",
			"record",
		},
		Params: []testing.Param{{
			ExtraData: []string{tabswitchcuj.WPRArchiveName},
			Val: tabswitchcuj.TabSwitchParam{
				BrowserType: browser.TypeAsh,
			},
			Pre: wpr.ReplayMode(tabswitchcuj.WPRArchiveName),
		}, {
			Name: "lacros",
			Val: tabswitchcuj.TabSwitchParam{
				BrowserType: browser.TypeLacros,
			},
			Fixture:           "tabSwitchCUJWPRLacros",
			ExtraSoftwareDeps: []string{"lacros"},
		}},
	})
}

func TabSwitchCUJ(ctx context.Context, s *testing.State) {
	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	// Wait for cpu to stabilize before test.
	if err := cpu.WaitUntilStabilized(ctx, cuj.CPUCoolDownConfig()); err != nil {
		// Log the cpu stabilizing wait failure instead of make it fatal.
		// TODO(b/213238698): Include the error as part of test data.
		s.Log("Failed to wait for CPU to become idle: ", err)
	}

	tabswitchcuj.Run(ctx, s)
}
