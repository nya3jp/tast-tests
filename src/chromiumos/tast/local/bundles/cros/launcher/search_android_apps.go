// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
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
		Func:         SearchAndroidApps,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Launches an Android app through the launcher",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
			"tbarzic@chromium.org",
			"cros-system-ui-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			Name:              "productivity_launcher_clamshell_mode",
			ExtraSoftwareDeps: []string{"android_p"},
			Val:               launcher.TestCase{ProductivityLauncher: true, TabletMode: false},
		}, {
			Name:              "clamshell_mode",
			ExtraSoftwareDeps: []string{"android_p"},
			Val:               launcher.TestCase{ProductivityLauncher: false, TabletMode: false},
		}, {
			Name:              "productivity_launcher_tablet_mode",
			Val:               launcher.TestCase{ProductivityLauncher: true, TabletMode: true},
			ExtraSoftwareDeps: []string{"android_p"},
			ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		}, {
			Name:              "tablet_mode",
			Val:               launcher.TestCase{ProductivityLauncher: false, TabletMode: true},
			ExtraSoftwareDeps: []string{"android_p"},
			ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		}, {
			Name:              "productivity_launcher_clamshell_mode_vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val:               launcher.TestCase{ProductivityLauncher: true, TabletMode: false},
		}, {
			Name:              "clamshell_mode_vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val:               launcher.TestCase{ProductivityLauncher: false, TabletMode: false},
		}, {
			Name:              "productivity_launcher_tablet_mode_vm",
			Val:               launcher.TestCase{ProductivityLauncher: true, TabletMode: true},
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		}, {
			Name:              "tablet_mode_vm",
			Val:               launcher.TestCase{ProductivityLauncher: false, TabletMode: true},
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		}},
		VarDeps: []string{"ui.gaiaPoolDefault"},
	})
}

// SearchAndroidApps tests launching an Android app from the Launcher.
func SearchAndroidApps(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	testCase := s.Param().(launcher.TestCase)
	tabletMode := testCase.TabletMode

	productivityLauncher := testCase.ProductivityLauncher
	var launcherFeatureOpt chrome.Option
	if productivityLauncher {
		launcherFeatureOpt = chrome.EnableFeatures("ProductivityLauncher")
	} else {
		launcherFeatureOpt = chrome.DisableFeatures("ProductivityLauncher")
	}

	cr, err := chrome.New(ctx,
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.ARCSupported(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...),
		launcherFeatureOpt)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	if err := optin.PerformAndClose(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to optin to Play Store and Close: ", err)
	}

	// Setup ARC.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(cleanupCtx)
	if err := a.WaitIntentHelper(ctx); err != nil {
		s.Fatal("Failed to wait for ARC Intent Helper: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

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

	if err := launcher.SearchAndWaitForAppOpen(tconn, kb, apps.PlayStore)(ctx); err != nil {
		s.Fatal("Failed to launch Play Store: ", err)
	}
}
