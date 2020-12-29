// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/ui/tabswitchcuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/wpr"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TabSwitchCUJ,
		Desc:         "Measures the performance of tab-switching CUJ",
		Contacts:     []string{"mukai@chromium.org", "tclaiborne@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{tabswitchcuj.WPRArchiveName},
		Timeout:      15 * time.Minute,
		Vars:         []string{"mute"},
		Pre:          wpr.ReplayMode(tabswitchcuj.WPRArchiveName),
	})
}

// TabSwitchCUJ measures the performance of tab-switching CUJ
func TabSwitchCUJ(ctx context.Context, s *testing.State) {
	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	cr, ok := s.PreValue().(*chrome.Chrome)
	if !ok {
		s.Fatal("Failed to connect to Chrome")
	}

	tabswitchcuj.Run(ctx, s, cr)
}
