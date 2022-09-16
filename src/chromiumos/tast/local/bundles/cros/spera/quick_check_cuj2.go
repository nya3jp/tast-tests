// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package spera

import (
	"context"
	"strconv"
	"time"

	"chromiumos/tast/common/cros/ui/setup"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/spera/quickcheckcuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/cuj"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/ui/cujrecorder"
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
		Contacts:     []string{"xliu@cienet.com", "alston.huang@cienet.com", "cienet-development@googlegroups.com"},
		SoftwareDeps: []string{"chrome", "arc", "wifi"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Vars: []string{
			"spera.cuj_mode",                 // Optional. Expecting "tablet" or "clamshell".
			"spera.collectTrace",             // Optional. Expecting "enable" or "disable", default is "disable".
			"spera.QuickCheckCUJ2_wait_time", // Optional. Given time for the system to stablize in seconds.
			"spera.QuickCheckCUJ2_wifissid",
			"spera.QuickCheckCUJ2_wifipassword",
		},
		Data: []string{cujrecorder.SystemTraceConfigFile},
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
				Fixture:           "loggedInAndKeepStateLacros",
				Timeout:           5 * time.Minute,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: quickCheckParam{
					tier:        cuj.Basic,
					scenario:    quickcheckcuj.Lock,
					browserType: browser.TypeLacros,
				},
			}, {
				Name:              "basic_unlock_crosbolt",
				Fixture:           "loggedInAndKeepState",
				Timeout:           5 * time.Minute,
				ExtraAttr:         []string{"group:crosbolt", "crosbolt_perbuild"},
				ExtraHardwareDeps: hwdep.D(setup.PerfCUJDevices()),
				Val: quickCheckParam{
					tier:     cuj.Basic,
					scenario: quickcheckcuj.Lock,
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
				Fixture:           "loggedInAndKeepStateLacros",
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
				ExtraHardwareDeps: hwdep.D(setup.PerfCUJDevices()),
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
	// The system may be unstable after login, causing suspending DUT operations to fail on some models.
	// Therefore, use a variable to control the startup delay time and find out the estimated time required by the model.
	// See b/234115114 for details.
	if wt, ok := s.Var("spera.QuickCheckCUJ2_wait_time"); ok {
		waitTime, err := strconv.Atoi(wt)
		if err != nil {
			s.Fatal("Failed to convert the spera.QuickCheckCUJ2_wait_time to integer")
		}
		s.Logf("Given %d seconds for the system to stablize", waitTime)
		if err := testing.Sleep(ctx, time.Duration(waitTime)*time.Second); err != nil {
			s.Fatalf("Failed to sleep for %d seconds: %v", waitTime, err)
		}
	}

	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	// Shorten context a bit to allow for cleanup if Run fails.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer cancel()

	var tabletMode bool
	if mode, ok := s.Var("spera.cuj_mode"); ok {
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

	pv := quickcheckcuj.Run(ctx, s, cr, scenario, tabletMode, param.browserType)
	if err := pv.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to saving perf data: ", err)
	}
}
