// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SearchBuiltInApps,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Launches a built-in app through the launcher",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
			"tbarzic@chromium.org",
			"cros-system-ui-eng@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name:    "productivity_launcher_clamshell_mode",
			Fixture: "chromeLoggedInWith100FakeAppsProductivityLauncher",
			Val:     launcher.TestCase{ProductivityLauncher: true, TabletMode: false},
			// b/229135388
			ExtraAttr: []string{"informational"},
		}, {
			Name:    "clamshell_mode",
			Fixture: "chromeLoggedInWith100FakeAppsLegacyLauncher",
			Val:     launcher.TestCase{ProductivityLauncher: false, TabletMode: false},
		}, {
			Name:              "productivity_launcher_tablet_mode",
			Fixture:           "chromeLoggedInWith100FakeAppsProductivityLauncher",
			Val:               launcher.TestCase{ProductivityLauncher: true, TabletMode: true},
			ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		}, {
			Name:              "tablet_mode",
			Fixture:           "chromeLoggedInWith100FakeAppsLegacyLauncher",
			Val:               launcher.TestCase{ProductivityLauncher: false, TabletMode: true},
			ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		}},
	})
}

// SearchBuiltInApps searches for the Settings app in the Launcher.
func SearchBuiltInApps(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)
	app := apps.Settings

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	testCase := s.Param().(launcher.TestCase)
	tabletMode := testCase.TabletMode

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, tabletMode)
	if err != nil {
		s.Fatal("Failed to ensure clamshell/tablet mode: ", err)
	}
	defer cleanup(cleanupCtx)

	// When a DUT switches from tablet mode to clamshell mode, sometimes it takes a while to settle down.
	if !tabletMode {
		if err := ash.WaitForLauncherState(ctx, tconn, ash.Closed); err != nil {
			s.Fatal("Launcher not closed after transition to clamshell mode: ", err)
		}
	}

	if err := launcher.SearchAndWaitForAppOpen(tconn, kb, app)(ctx); err != nil {
		s.Fatal("Failed to launch app: ", err)
	}
}
