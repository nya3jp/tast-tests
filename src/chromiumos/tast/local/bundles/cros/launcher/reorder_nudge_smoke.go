// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ReorderNudgeSmoke,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify the reorder nudge's behaviors",
		Contacts: []string{
			"andrewxu@chromium.org",
			"chromeos-sw-engprod@google.com",
			"cros-system-ui-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      3 * time.Minute,
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val:  launcher.TestCase{TabletMode: false},
		}, {
			Name:              "tablet_mode",
			Val:               launcher.TestCase{TabletMode: true},
			ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		}},
	})
}

// ReorderNudgeSmoke verifies the reorder nudge stops showing up after 3 times.
func ReorderNudgeSmoke(ctx context.Context, s *testing.State) {
	testParam := s.Param().(launcher.TestCase)
	tabletMode := testParam.TabletMode

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Enforce the DUT to start in the expected mode to avoid mode switching.
	var enforceModeFlag string
	if tabletMode {
		enforceModeFlag = "--force-tablet-mode=touch_view"
	} else {
		enforceModeFlag = "--force-tablet-mode=clamshell"
	}

	cr, err := chrome.New(ctx, chrome.EnableFeatures("LauncherAppSort"), chrome.ExtraArgs("--skip-reorder-nudge-show-threshold-duration", enforceModeFlag))
	if err != nil {
		s.Fatalf("Failed to start the chrome with %s: %v", enforceModeFlag, err)
	}
	defer cr.Close(cleanupCtx)

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	ui := uiauto.New(tconn)
	isBubbleLauncher := !tabletMode
	var launcherType string
	if isBubbleLauncher {
		launcherType = "the bubble launcher"
	} else {
		launcherType = "the tablet launcher"
	}

	// The reorder nudge should show when opening the launcher for the first three times. Then it should not show at the fourth time.
	action := uiauto.Combine("open then close "+launcherType+" 4 times",
		launcher.ShowLauncher(tconn, isBubbleLauncher),
		ui.WaitUntilExists(launcher.ReorderEducationNudgeFinder),
		launcher.HideLauncher(tconn, isBubbleLauncher),
		launcher.ShowLauncher(tconn, isBubbleLauncher),
		ui.WaitUntilExists(launcher.ReorderEducationNudgeFinder),
		launcher.HideLauncher(tconn, isBubbleLauncher),
		launcher.ShowLauncher(tconn, isBubbleLauncher),
		ui.WaitUntilExists(launcher.ReorderEducationNudgeFinder),
		launcher.HideLauncher(tconn, isBubbleLauncher),
		launcher.ShowLauncher(tconn, isBubbleLauncher),
		ui.WaitUntilGone(launcher.ReorderEducationNudgeFinder),
		launcher.HideLauncher(tconn, isBubbleLauncher),
	)

	if err := action(ctx); err != nil {
		s.Fatal("Filed to open then hide the launcher for 4 times and verify the reorder nudge visibility: ", err)
	}
}
