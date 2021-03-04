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
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: LaunchHelpAppInBrowserMenu,
		Desc: "Help app can be launched in browser menu",
		Contacts: []string{
			"showoff-eng@google.com",
			"shengjun@chromium.org", // original test author
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedInForEA",
		Params: []testing.Param{
			{
				Name:              "stable",
				ExtraHardwareDeps: pre.AppsStableModels,
			}, {
				Name:              "unstable",
				ExtraHardwareDeps: pre.AppsUnstableModels,
				ExtraAttr:         []string{"informational"},
			},
		},
	})
}

// LaunchHelpAppInBrowserMenu verifies launching Help app in chrome browser three dot menu.
func LaunchHelpAppInBrowserMenu(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	conn, err := cr.NewConn(ctx, chrome.BlankURL)
	if err != nil {
		s.Fatal("Failed to lunch a new browser: ", err)
	}
	defer conn.Close()

	opts := &testing.PollOptions{Timeout: 10 * time.Second, Interval: 1 * time.Second}

	if err := ui.StableFindAndClick(ctx, tconn, ui.FindParams{
		Role:      ui.RoleTypePopUpButton,
		ClassName: "BrowserAppMenuButton",
	}, opts); err != nil {
		s.Fatal("Failed to click Chrome browser menu button")
	}

	if err := ui.StableFindAndClick(ctx, tconn, ui.FindParams{
		Name:      "Help",
		ClassName: "MenuItemView",
	}, opts); err != nil {
		s.Fatal("Failed to click Help item in browser menu options")
	}

	if err := ui.StableFindAndClick(ctx, tconn, ui.FindParams{
		Name:      "Get Help",
		ClassName: "MenuItemView",
	}, opts); err != nil {
		s.Fatal("Failed to click Get Help under Help sub menu")
	}

	if err := helpapp.WaitForApp(ctx, tconn); err != nil {
		s.Fatal("Failed to launch or render Help app from Chrome browser menu: ", err)
	}
}
