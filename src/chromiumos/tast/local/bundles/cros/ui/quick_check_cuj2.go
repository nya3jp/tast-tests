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
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type quickCheckParam struct {
	tier     cuj.Tier
	scenario quickcheckcuj.PauseMode
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         QuickCheckCUJ2,
		Desc:         "Measures the system performance after login or wakeup by checking common apps",
		Contacts:     []string{"xliu@cienet.com", "hc.tsai@cienet.com", "alfredyu@cienet.com"},
		SoftwareDeps: []string{"chrome", "arc", "wifi"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Fixture:      "loggedInAndKeepState",
		Vars: []string{
			"ui.cuj_mode",     // Optional. Expecting "tablet" or "clamshell".
			"ui.cuj_password", // Required to unlock screen.
			"ui.QuickCheckCUJ2_wifissid",
			"ui.QuickCheckCUJ2_wifipassword",
		},
		Params: []testing.Param{
			{
				Name:    "basic_unlock",
				Timeout: 5 * time.Minute,
				Val: quickCheckParam{
					tier:     cuj.Basic,
					scenario: quickcheckcuj.Lock,
				},
			}, {
				Name:    "basic_wakeup",
				Timeout: 5 * time.Minute,
				Val: quickCheckParam{
					tier:     cuj.Basic,
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

	// Shorten context a bit to allow for cleanup if Run fails.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer cancel()

	var tabletMode bool
	if mode, ok := s.Var("ui.cuj_mode"); ok {
		tabletMode = mode == "tablet"
		cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, tabletMode)
		if err != nil {
			s.Fatalf("Failed to enable tablet mode to %v: %v", tabletMode, err)
		}
		defer cleanup(cleanupCtx)
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

	// Ensure keyboard layout is "US ENG" so password can be entered correctly when unlocking screen.
	if err := ime.EnglishUS.Activate(tconn)(ctx); err != nil {
		s.Fatal("Failed to set keyboard layout to US ENG: ", err)
	}

	pv := quickcheckcuj.Run(ctx, s, cr, scenario, tabletMode)
	if err := pv.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to saving perf data: ", err)
	}
}
