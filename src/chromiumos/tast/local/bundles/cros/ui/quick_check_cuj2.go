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
	"chromiumos/tast/local/bundles/cros/ui/setup"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type quickCheckParam struct {
	tier        cuj.Tier
	scenario    quickcheckcuj.PauseMode
	browserType browser.Type
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         QuickCheckCUJ2,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Measures the system performance after login or wakeup by checking common apps",
		Contacts:     []string{"xliu@cienet.com", "hc.tsai@cienet.com", "alfredyu@cienet.com"},
		SoftwareDeps: []string{"chrome", "arc", "wifi"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Vars: []string{
			"ui.cuj_mode", // Optional. Expecting "tablet" or "clamshell".
			"ui.QuickCheckCUJ2_wifissid",
			"ui.QuickCheckCUJ2_wifipassword",
		},
		Params: []testing.Param{
			{
				Name:    "basic_unlock",
				Fixture: "loggedInAndKeepState",
				Timeout: 5 * time.Minute,
				Val: quickCheckParam{
					tier:     cuj.Basic,
					scenario: quickcheckcuj.Lock,
				},
			}, {
				Name:              "basic_lacros_unlock",
				Fixture:           "loggedInAndKeepStateLacrosWithARC",
				Timeout:           5 * time.Minute,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: quickCheckParam{
					tier:        cuj.Basic,
					scenario:    quickcheckcuj.Lock,
					browserType: browser.TypeLacros,
				},
			}, {
				Name:    "basic_wakeup",
				Fixture: "loggedInAndKeepState",
				Timeout: 5 * time.Minute,
				Val: quickCheckParam{
					tier:     cuj.Basic,
					scenario: quickcheckcuj.Suspend,
				},
			}, {
				Name:              "basic_lacros_wakeup",
				Fixture:           "loggedInAndKeepStateLacrosWithARC",
				Timeout:           5 * time.Minute,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: quickCheckParam{
					tier:        cuj.Basic,
					scenario:    quickcheckcuj.Suspend,
					browserType: browser.TypeLacros,
				},
			}, {
				Name:              "basic_wakeup_crosbolt",
				Fixture:           "loggedInAndKeepState",
				Timeout:           5 * time.Minute,
				ExtraAttr:         []string{"group:crosbolt", "crosbolt_perbuild"},
				ExtraHardwareDeps: hwdep.D(setup.PerfCUJBasicDevices()),
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
	p := s.Param().(quickCheckParam)
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	var lacrosFixtValue lacrosfixt.FixtValue
	if p.browserType == browser.TypeLacros {
		lacrosFixtValue = s.FixtValue().(cuj.FixtureData).LacrosFixt
	}
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
	if tabletMode {
		cleanup, err := display.RotateToLandscape(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to rotate display to landscape: ", err)
		}
		defer cleanup(cleanupCtx)
	}

	param := s.Param().(quickCheckParam)
	scenario := param.scenario

	pv := quickcheckcuj.Run(ctx, s, cr, scenario, tabletMode, lacrosFixtValue)
	if err := pv.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to saving perf data: ", err)
	}
}
