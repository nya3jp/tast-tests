// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"context"

	"chromiumos/tast/local/bundles/cros/apps/helpapp"
	"chromiumos/tast/local/bundles/cros/apps/pre"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
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

	ui := uiauto.New(tconn)

	browserAppMenuButtonFinder := nodewith.ClassName("BrowserAppMenuButton").Role(role.PopUpButton)
	helpMenuItemFinder := nodewith.ClassName("MenuItemView").Name("Help")
	getHelpMenuItemFinder := nodewith.ClassName("MenuItemView").Name("Get Help")

	if err := uiauto.Combine("launch Help app from Chrome browser",
		ui.LeftClick(browserAppMenuButtonFinder),
		ui.LeftClick(helpMenuItemFinder),
		ui.LeftClick(getHelpMenuItemFinder),
		helpapp.NewContext(cr, tconn).WaitForApp(),
	)(ctx); err != nil {
		s.Fatal("Failed to launch or render Help app from Chrome browser menu: ", err)
	}
}
