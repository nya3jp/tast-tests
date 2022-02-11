// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/apps/helpapp"
	"chromiumos/tast/local/bundles/cros/apps/pre"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// testParameters contains all the data needed to run a single test iteration.
type testParameters struct {
	tabletMode bool
	oobe       bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         LaunchHelpApp,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Help app should be launched after OOBE",
		Contacts: []string{
			"showoff-eng@google.com",
			"shengjun@chromium.org",
		},
		Attr:         []string{"group:mainline"},
		VarDeps:      []string{"ui.gaiaPoolDefault"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Timeout:      chrome.GAIALoginTimeout + time.Minute,
		Params: []testing.Param{
			{
				Name:              "clamshell_oobe_stable",
				ExtraHardwareDeps: hwdep.D(pre.AppsStableModels),
				Val: testParameters{
					tabletMode: false,
					oobe:       true,
				},
			}, {
				Name:              "clamshell_oobe_unstable",
				ExtraHardwareDeps: hwdep.D(pre.AppsUnstableModels),
				ExtraAttr:         []string{"informational"},
				Val: testParameters{
					tabletMode: true,
					oobe:       true,
				},
			}, {
				Name:              "tablet_oobe_stable",
				ExtraHardwareDeps: hwdep.D(pre.AppsStableModels),
				Val: testParameters{
					tabletMode: true,
					oobe:       true,
				},
			}, {
				Name:              "tablet_oobe_unstable",
				ExtraHardwareDeps: hwdep.D(pre.AppsUnstableModels),
				ExtraAttr:         []string{"informational"},
				Val: testParameters{
					tabletMode: true,
					oobe:       true,
				},
			}, {
				Name:              "clamshell_logged_in_stable",
				ExtraHardwareDeps: hwdep.D(pre.AppsStableModels),
				Val: testParameters{
					tabletMode: false,
					oobe:       false,
				},
				Fixture: "chromeLoggedInForEA",
			}, {
				Name:              "clamshell_logged_in_unstable",
				ExtraHardwareDeps: hwdep.D(pre.AppsUnstableModels),
				ExtraAttr:         []string{"informational"},
				Val: testParameters{
					tabletMode: true,
					oobe:       true,
				},
			}, {
				Name:              "tablet_logged_in_stable",
				ExtraHardwareDeps: hwdep.D(pre.AppsStableModels, hwdep.TouchScreen()),
				Val: testParameters{
					tabletMode: true,
					oobe:       false,
				},
				Fixture: "chromeLoggedInForEA",
			}, {
				Name:              "tablet_logged_in_unstable",
				ExtraHardwareDeps: hwdep.D(pre.AppsUnstableModels, hwdep.TouchScreen()),
				ExtraAttr:         []string{"informational"},
				Val: testParameters{
					tabletMode: true,
					oobe:       false,
				},
				Fixture: "chromeLoggedInForEA",
			},
		}})
}

// LaunchHelpApp verifies launching Showoff after OOBE.
func LaunchHelpApp(ctx context.Context, s *testing.State) {
	if s.Param().(testParameters).oobe {
		helpAppLaunchDuringOOBE(ctx, s, s.Param().(testParameters).tabletMode)
	} else {
		helpAppLaunchAfterLogin(ctx, s, s.Param().(testParameters).tabletMode)
	}
}

// helpAppLaunchDuringOOBE verifies help app launch during OOBE stage. Help app only launches with real user login in clamshell mode.
func helpAppLaunchDuringOOBE(ctx context.Context, s *testing.State, isTabletMode bool) {
	var uiMode string
	if isTabletMode {
		uiMode = "--force-tablet-mode=touch_view"
	} else {
		uiMode = "--force-tablet-mode=clamshell"
	}

	cr, err := chrome.New(ctx,
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.DontSkipOOBEAfterLogin(),
		chrome.EnableFeatures("HelpAppFirstRun"),
		chrome.ExtraArgs(uiMode))

	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Verify HelpApp (aka Explore) launched in Clamshell mode only.
	if err := assertHelpAppLaunched(ctx, s, tconn, cr, !isTabletMode); err != nil {
		s.Fatalf("Failed to verify help app launching during oobe in tablet mode enabled(%v): %v", isTabletMode, err)
	}
}

// helpAppLaunchAfterLogin verifies help app launch after user login. It should be able to launch on devices in both clamshell and tablet mode.
func helpAppLaunchAfterLogin(ctx context.Context, s *testing.State, isTabletMode bool) {
	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	s.Logf("Ensure tablet mode enabled(%v)", isTabletMode)
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, isTabletMode)
	if err != nil {
		s.Fatalf("Failed to ensure tablet mode enabled(%v): %v", isTabletMode, err)
	}
	defer cleanup(ctx)

	if err := helpapp.NewContext(cr, tconn).Launch()(ctx); err != nil {
		s.Fatal("Failed to launch help app: ", err)
	}

	if err := assertHelpAppLaunched(ctx, s, tconn, cr, true); err != nil {
		s.Fatal("Failed to verify help app launching after user logged in: ", err)
	}
}

// assertHelpAppLaunched asserts help app to be launched or not
func assertHelpAppLaunched(ctx context.Context, s *testing.State, tconn *chrome.TestConn, cr *chrome.Chrome, isLaunched bool) error {
	helpCtx := helpapp.NewContext(cr, tconn)
	if isLaunched {
		// Verify perk is shown to default consumer user.
		isPerkShown, err := helpCtx.IsHTMLElementPresent(ctx, "showoff-offers-page")
		if err != nil {
			s.Fatal("Failed to evaluate offers page: ", err)
		}

		if !isPerkShown {
			s.Error("Perk is not shown to a consumer user")
		}
	} else {
		isAppLaunched, err := helpCtx.Exists(ctx)
		if err != nil {
			s.Fatal("Failed to verify help app existence: ", err)
		}

		if isAppLaunched {
			s.Fatal("Help app should not be launched after oobe on a managed device")
		}
	}
	return nil
}

// shouldLaunchHelp returns a result to launch help app or not.
func shouldLaunchHelp(isTabletMode, isOOBE bool) bool {
	return !isOOBE || !isTabletMode
}
