// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Smoke,
		Desc: "Opens launcher either using launcher button, or a keyboard shortcut",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
			"tbarzic@chromium.org",
			"cros-system-ui-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
	})
}

// Smoke opens launcher using keyboard shortcut, or launcher button.
func Smoke(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// The test expects clamshell mode.
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure clamshell/tablet mode: ", err)
	}
	defer cleanup(ctx)

	// When a DUT switches from tablet mode to clamshell mode, sometimes it takes a while to settle down.
	// Added a delay here to let all events finishing up.
	if err := ui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for location changes: ", err)
	}

	// Open peeking launcher using search key accelerator.
	if err := ash.TriggerLauncherStateChange(ctx, tconn, ash.AccelSearch); err != nil {
		s.Fatal("Failed to open launcher: ", err)
	}
	if err := ash.WaitForLauncherState(ctx, tconn, ash.Peeking); err != nil {
		s.Fatal("Failed to wait for peeking launcher state: ", err)
	}

	ui := uiauto.New(tconn)

	// Click expand arrow to transition launcher to fullscreen state.
	if err := ui.LeftClick(nodewith.ClassName("ExpandArrowView"))(ctx); err != nil {
		s.Fatal("Could not click expand arrow button: ", err)
	}
	if err := ash.WaitForLauncherState(ctx, tconn, ash.FullscreenAllApps); err != nil {
		s.Fatal("Failed to wait for fullscreen apps launcher state: ", err)
	}

	// Search accelerator closes the launcher.
	if err := ash.TriggerLauncherStateChange(ctx, tconn, ash.AccelSearch); err != nil {
		s.Fatal("Failed to close launcher: ", err)
	}
	if err := ash.WaitForLauncherState(ctx, tconn, ash.Closed); err != nil {
		s.Fatal("Failed to wait for closed launcher state: ", err)
	}

	// Click the home button to open peeking launcher.
	if err := ui.LeftClick(nodewith.ClassName("ash/HomeButton"))(ctx); err != nil {
		s.Fatal("Could not click the home button: ", err)
	}
	if err := ash.WaitForLauncherState(ctx, tconn, ash.Peeking); err != nil {
		s.Fatal("Failed to wait for peeking launcher state: ", err)
	}

	// Click expand arrow to transition launcher to fullscreen state.
	if err := ui.LeftClick(nodewith.ClassName("ExpandArrowView"))(ctx); err != nil {
		s.Fatal("Could not click expand arrow button: ", err)
	}
	if err := ash.WaitForLauncherState(ctx, tconn, ash.FullscreenAllApps); err != nil {
		s.Fatal("Failed to wait for fullscreen apps launcher state: ", err)
	}

	// Click the home button to close launcher.
	if err := ui.LeftClick(nodewith.ClassName("ash/HomeButton"))(ctx); err != nil {
		s.Fatal("Could not click the home button: ", err)
	}
	if err := ash.WaitForLauncherState(ctx, tconn, ash.Closed); err != nil {
		s.Fatal("Failed to wait for closed launcher state: ", err)
	}

	// Shift-search accelerator opens the launcher in fullscreen state.
	if err := ash.TriggerLauncherStateChange(ctx, tconn, ash.AccelShiftSearch); err != nil {
		s.Fatal("Failed to open launcher: ", err)
	}
	if err := ash.WaitForLauncherState(ctx, tconn, ash.FullscreenAllApps); err != nil {
		s.Fatal("Failed to wait for fullscreen apps launcher state: ", err)
	}

	// Shift-search accelerator closes the launcher.
	if err := ash.TriggerLauncherStateChange(ctx, tconn, ash.AccelShiftSearch); err != nil {
		s.Fatal("Failed to close launcher: ", err)
	}
	if err := ash.WaitForLauncherState(ctx, tconn, ash.Closed); err != nil {
		s.Fatal("Failed to wait for closed launcher state: ", err)
	}
}
