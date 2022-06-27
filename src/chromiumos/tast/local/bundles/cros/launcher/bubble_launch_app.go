// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/testing"
)

type bubbleLaunchAppTestType string

const (
	enableLauncherAppSort  bubbleLaunchAppTestType = "EnableLauncherAppSort"  // Enable "LauncherAppSort" feature in the test
	disableLauncherAppSort bubbleLaunchAppTestType = "DisableLauncherAppSort" // Disable "LauncherAppSort" feature in the test
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BubbleLaunchApp,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests that the bubble launcher closes when opening an app",
		Contacts: []string{
			"cros-system-ui-eng@google.com",
			"chromeos-sw-engprod@google.com",
			"jamescook@chromium.org",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{
			{
				Name:    "enable_launcher_app_sort",
				Val:     enableLauncherAppSort,
				Fixture: "chromeLoggedInWith100FakeAppsProductivityLauncherAppSort",
			},
			{
				Name:    "disable_launcher_app_sort",
				Val:     disableLauncherAppSort,
				Fixture: "chromeLoggedInWith100FakeAppsProductivityLauncher",
			},
		},
	})
}

func BubbleLaunchApp(ctx context.Context, s *testing.State) {
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

	// Ensure bubble launcher is open.
	if err := launcher.OpenBubbleLauncher(tconn)(ctx); err != nil {
		s.Fatal("Failed to open bubble launcher: ", err)
	}

	// Ensure apps are finished installing.
	if err := launcher.WaitForStableNumberOfApps(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for item count in app list to stabilize: ", err)
	}

	ui := uiauto.New(tconn)
	bubble := nodewith.ClassName(ash.AppListBubbleClassName)
	settingButton := nodewith.Role(role.Button).Name(apps.Settings.Name).Ancestor(nodewith.ClassName(launcher.BubbleAppsGridViewClass))

	if s.Param().(bubbleLaunchAppTestType) == enableLauncherAppSort {
		// When the launcher app sort feature is enabled, fake apps are placed at the front. In this case, scroll the apps grid to the end to show the setting app button before launching the app.
		if err := ui.MakeVisible(settingButton)(ctx); err != nil {
			s.Fatal("Failed to make the settings button visible: ", err)
		}

		// Wait for the settings app to appear onscreen.
		if err := waitForOnscreen(ctx, ui, settingButton); err != nil {
			s.Fatal("Failed to wait for settings app to appear onscreen: ", err)
		}

		// Wait until the setting app button's bounds become stable.
		if err := ui.WaitForLocation(settingButton)(ctx); err != nil {
			s.Fatal("Failed to wait for the setting app button location changes: ", err)
		}
	}

	if err := uiauto.Combine("close bubble by launching Settings app",
		ui.LeftClick(settingButton),
		ui.WaitUntilGone(bubble),
	)(ctx); err != nil {
		s.Fatal("Could not close bubble by launching Settings app: ", err)
	}

	s.Log("Waiting for Settings app to launch")
	if err := ash.WaitForApp(ctx, tconn, apps.Settings.ID, time.Minute); err != nil {
		s.Error("Settings app did not start from bubble launcher: ", err)
	}
}

func waitForOnscreen(ctx context.Context, ui *uiauto.Context, targetItem *nodewith.Finder) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		info, err := ui.Info(ctx, targetItem)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get target item info"))
		}
		if info.State[state.Offscreen] {
			return errors.New("Item is offscreen")
		}
		return nil

	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: 500 * time.Millisecond})
}
