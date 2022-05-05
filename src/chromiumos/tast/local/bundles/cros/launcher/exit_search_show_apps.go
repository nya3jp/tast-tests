// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ExitSearchShowApps,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks for best match search results in the launcher",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
			"yulunwu@chromium.org",
			"cros-system-ui-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name:    "clamshell_mode",
			Fixture: "chromeLoggedInWithProductivityLauncher",
			Val:     launcher.TestCase{TabletMode: false},
		}, {
			Name:              "tablet_mode",
			Fixture:           "chromeLoggedInWithProductivityLauncher",
			Val:               launcher.TestCase{TabletMode: true},
			ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		}},
	})
}

type exitSearchShowAppsTestCase struct {
	subtestName      string
	exitSearchAction uiauto.Action
}

// ExitSearchShowApps checks that exiting the productivity launcher search UI
// transitions the user back to the app list UI.
func ExitSearchShowApps(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

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

	if !tabletMode {
		if err := ash.WaitForLauncherState(ctx, tconn, ash.Closed); err != nil {
			s.Fatal("Launcher not closed: ", err)
		}
	}

	ui := uiauto.New(tconn)

	subtests := []exitSearchShowAppsTestCase{
		{
			subtestName:      "Escape Key",
			exitSearchAction: kb.TypeKeyAction(input.KEY_ESC),
		},
		{
			subtestName:      "Close Button",
			exitSearchAction: ui.LeftClick(nodewith.ClassName("SearchBoxImageButton")),
		},
		{
			subtestName: "Multiple Backspace",
			exitSearchAction: uiauto.Combine("Multiple Backspace",
				kb.TypeKeyAction(input.KEY_BACKSPACE),
				kb.TypeKeyAction(input.KEY_BACKSPACE),
				kb.TypeKeyAction(input.KEY_BACKSPACE),
				kb.TypeKeyAction(input.KEY_BACKSPACE)),
		},
	}

	for _, subtest := range subtests {
		s.Run(ctx, subtest.subtestName, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+string(subtest.subtestName))

			if err := uiauto.Combine("search launcher",
				launcher.Open(tconn),
				launcher.Search(tconn, kb, "test"),
				subtest.exitSearchAction,
				launcher.WaitForLauncherSearchExit(tconn, tabletMode),
			)(ctx); err != nil {
				s.Fatal("Failed to enter and exit search: ", err)
			}
		})
	}

}
