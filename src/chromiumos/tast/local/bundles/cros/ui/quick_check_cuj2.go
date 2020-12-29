// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/bundles/cros/ui/quickcheckcuj"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type quickCheckParam struct {
	tire     cuj.Tier
	scenario quickcheckcuj.PauseMode
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         QuickCheckCUJ2,
		Desc:         "Measures the system performance after login or wakeup by checking common apps",
		Contacts:     []string{"xliu@cienet.com", "hc.tsai@cienet.com", "alfredyu@cienet.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "arc", "wifi"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Fixture:      "loggedInAndKeepState",
		Vars: []string{
			"mode", // Expecting "tablet" or "clamshell".
			"ui.cuj_password",
			"ui.cuj_wifissid",
			"ui.cuj_wifipassword",
		},
		Params: []testing.Param{
			{
				Name:    "basic_unlock",
				Timeout: 10 * time.Minute,
				Val: quickCheckParam{
					tire:     cuj.Basic,
					scenario: quickcheckcuj.Lock,
				},
			}, {
				Name:    "basic_wakeup",
				Timeout: 10 * time.Minute,
				Val: quickCheckParam{
					tire:     cuj.Basic,
					scenario: quickcheckcuj.Suspend,
				},
			},
		},
	})
}

// QuickCheckCUJ2 measures the system performance after login or wakeup by checking common apps
func QuickCheckCUJ2(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(cuj.FixtureData).Chrome
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	var tabletMode bool
	if mode, ok := s.Var("mode"); ok {
		tabletMode = mode == "tablet"
		cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, tabletMode)
		if err != nil {
			s.Fatalf("Failed to enable tablet mode to %v: %v", tabletMode, err)
		}
		defer cleanup(ctx)
	} else {
		// Use default screen mode of the DUT.
		tabletMode, err = ash.TabletModeEnabled(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get DUT default screen mode: ", err)
		}
	}
	s.Log("Running test with tablet mode: ", tabletMode)

	param := s.Param().(quickCheckParam)
	scenario := param.scenario

	// Shorten context a bit to allow for cleanup if Run fails.
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer cancel()

	pv := quickcheckcuj.Run(ctx, s, cr, scenario, tabletMode)
	if err := pv.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to saving perf data: ", err)
	}
}
