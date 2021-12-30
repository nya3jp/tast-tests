// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BubbleSmoke,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Basic smoke tests for the bubble launcher",
		Contacts: []string{
			"cros-system-ui-eng@google.com",
			"chromeos-sw-engprod@google.com",
			"jamescook@chromium.org",
		},
		Attr:         []string{"group:mainline"},
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

	ui := uiauto.New(tconn)
	bubble := nodewith.ClassName(ash.AppListBubbleClassName)

	// When a DUT switches from tablet mode to clamshell mode, sometimes it
	// takes a while to settle down. Wait for the transition to finish.
	if err := ui.WaitForLocation(nodewith.Root())(ctx); err != nil {
		s.Fatal("Failed to wait for location changes: ", err)
	}

	if err := uiauto.Combine("open bubble by clicking home button",
		ui.LeftClick(nodewith.ClassName("ash/HomeButton")),
		ui.WaitUntilExists(bubble),
	)(ctx); err != nil {
		s.Fatal("Could not open bubble by clicking home button: ", err)
	}

	if err := uiauto.Combine("close bubble by clicking in screen corner",
		mouse.Click(tconn, coords.Point{X: 0, Y: 0}, mouse.LeftButton),
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

	if err := uiauto.Combine("close bubble by launching Settings app",
		ui.LeftClick(
			nodewith.Role(role.Button).Name(apps.Settings.Name).Ancestor(bubble)),
		ui.WaitUntilGone(bubble),
	)(ctx); err != nil {
		s.Fatal("Could not close bubble by launching Settings app: ", err)
	}

	s.Log("Waiting for Settings app to launch")
	if err := ash.WaitForApp(ctx, tconn, apps.Settings.ID, time.Minute); err != nil {
		s.Fatal("Settings app did not start from bubble launcher: ", err)
	}
}
