// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BubbleSmoke,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Basic smoke tests for the bubble launcher",
		Contacts: []string{
			"cros-system-ui-eng@google.com",
			"chromeos-sw-engprod@google.com",
			"tbarzic@chromium.org",
		},
		// TODO(https://crbug.com/1255265): Remove "informational" once stable.
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedInWith100FakeAppsProductivityLauncher",
	})
}

func BubbleSmoke(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree")

	// Bubble launcher requires clamshell mode.
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure clamshell mode: ", err)
	}
	defer cleanup(ctx)

	if err := ash.WaitForLauncherState(ctx, tconn, ash.Closed); err != nil {
		s.Fatal("Launcher not closed: ", err)
	}

	ui := uiauto.New(tconn)
	bubble := nodewith.ClassName(ash.AppListBubbleClassName)
	if err := uiauto.Combine("open bubble by clicking home button",
		ui.LeftClick(nodewith.ClassName("ash/HomeButton")),
		ui.WaitUntilExists(bubble),
	)(ctx); err != nil {
		s.Fatal("Could not open bubble by clicking home button: ", err)
	}

	// Click close to the corner, but not exactly at origin to avoid the coordinate being transformed to
	// bounds outside the root window at it's piped to UI.
	// TODO(b/221688041): Consider changing back to (0,0) if the crash linked to the bug stops happening.
	if err := uiauto.Combine("close bubble by clicking in screen corner",
		mouse.Click(tconn, coords.Point{X: 4, Y: 4}, mouse.LeftButton),
		ui.WaitUntilGone(bubble),
	)(ctx); err != nil {
		s.Fatal("Could not close bubble by clicking in screen corner: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	if err := kb.Accel(ctx, "Search"); err != nil {
		s.Fatal("Failed to press Search: ", err)
	}

	if err := ui.WaitUntilExists(bubble)(ctx); err != nil {
		s.Fatal("Could not reopen bubble by pressing Search key: ", err)
	}

	if err := kb.Accel(ctx, "Search"); err != nil {
		s.Fatal("Failed to press Search again: ", err)
	}

	if err := ui.WaitUntilGone(bubble)(ctx); err != nil {
		s.Error("Could not close bubble by pressing Search key again: ", err)
	}
}
