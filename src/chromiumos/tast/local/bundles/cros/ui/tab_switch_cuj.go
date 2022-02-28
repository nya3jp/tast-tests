// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/bundles/cros/ui/tabswitchcuj"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/cpu"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/wpr"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TabSwitchCUJ,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Measures the performance of tab-switching CUJ",
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		Contacts:     []string{"mukai@chromium.org", "tclaiborne@chromium.org", "chromeos-wmp@google.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      22*time.Minute + cuj.CPUStablizationTimeout,
		Vars:         []string{"mute"},
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
		}, {
			Name:      "trace",
			ExtraData: []string{tabswitchcuj.WPRArchiveName},
			Val: tabswitchcuj.TabSwitchParam{
				BrowserType: browser.TypeAsh,
				Tracing:     true,
			},
			Pre: wpr.ReplayMode(tabswitchcuj.WPRArchiveName),
		}, {
			Name:      "validation",
			ExtraData: []string{tabswitchcuj.WPRArchiveName},
			Val: tabswitchcuj.TabSwitchParam{
				BrowserType: browser.TypeAsh,
				Validation:  true,
			},
			Pre: wpr.ReplayMode(tabswitchcuj.WPRArchiveName),
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
