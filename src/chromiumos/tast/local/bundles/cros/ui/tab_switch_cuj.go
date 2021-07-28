// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/ui/tabswitchcuj"
	"chromiumos/tast/local/lacros"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/wpr"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TabSwitchCUJ,
		Desc:         "Measures the performance of tab-switching CUJ",
		Contacts:     []string{"mukai@chromium.org", "tclaiborne@chromium.org", "chromeos-wmp@google.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      22 * time.Minute,
		Vars:         []string{"mute"},
		Params: []testing.Param{{
			ExtraAttr: []string{"group:crosbolt", "crosbolt_perbuild"},
			ExtraData: []string{tabswitchcuj.WPRArchiveName},
			Val:       lacros.ChromeTypeChromeOS,
			Pre:       wpr.ReplayMode(tabswitchcuj.WPRArchiveName),
		}, {
			Name:              "lacros",
			Val:               lacros.ChromeTypeLacros,
			Fixture:           "loggedInToCUJUserLacros",
			ExtraSoftwareDeps: []string{"lacros"},
		}},
	})
}

func TabSwitchCUJ(ctx context.Context, s *testing.State) {
	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	tabswitchcuj.Run(ctx, s)
}
