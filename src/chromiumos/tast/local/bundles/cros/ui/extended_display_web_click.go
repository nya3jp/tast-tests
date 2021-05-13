// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ExtendedDisplayWebClick,
		Desc:         "A test to debug the click issue on extended display web page",
		Contacts:     []string{"xliu@cienet.com"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Fixture:      "chromeLoggedIn",
	})
}

func ExtendedDisplayWebClick(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	defer faillog.SaveScreenshotOnError(ctx, cr, s.OutDir(), s.HasError)

	// Make sure there are two displays on DUT.
	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get display info: ", err)
	}

	if len(infos) != 2 {
		s.Fatal("Expect 2 displays but got ", len(infos))
	}

	ui := uiauto.New(tconn)
	// Root window on internal display.
	window1 := nodewith.ClassName("RootWindow-0").Role(role.Window)
	// Root window on extended display.
	window2 := nodewith.ClassName("RootWindow-1").Role(role.Window)
	winInfo1, err := ui.Info(ctx, window1)
	if err != nil {
		s.Fatal("Failed to get windows 1 info: ", err)
	}
	testing.ContextLog(ctx, "Location of window 1: ", winInfo1.Location)
	winInfo2, err := ui.Info(ctx, window2)
	if err != nil {
		s.Fatal("Failed to get windows 2 info: ", err)
	}
	testing.ContextLog(ctx, "Location of window 2: ", winInfo2.Location)

	testing.ContextLog(ctx, "Open chrome from extended display")
	chromeIcon := nodewith.Name("Google Chrome").ClassName("ash/ShelfAppButton").Ancestor(window2)
	if err := ui.LeftClick(chromeIcon)(ctx); err != nil {
		s.Fatal("Failed to open chrome browser from extended display: ", err)
	}

	testing.ContextLog(ctx, "Navigate to google.com")
	_, err = cr.NewConn(ctx, "https://google.com")
	if err != nil {
		s.Fatal("Failed to open new tab: ", err)
	}

	browser := nodewith.ClassName("BrowserFrame").Ancestor(window2)
	browserInfo, err := ui.Info(ctx, browser)
	if err != nil {
		s.Fatal("Failed to get browser info: ", err)
	}
	testing.ContextLog(ctx, "Location of browser: ", browserInfo.Location)

	searchBox := nodewith.Name("Search").Role(role.TextFieldWithComboBox).Ancestor(browser)
	searchBoxInfo, err := ui.Info(ctx, searchBox)
	if err != nil {
		s.Fatal("Failed to get search box info: ", err)
	}
	testing.ContextLog(ctx, "Location of search box: ", searchBoxInfo.Location)

	testing.ContextLog(ctx, "Move mouse over to search box")
	if err := ui.MouseMoveTo(searchBox, 3*time.Second)(ctx); err != nil {
		s.Fatal("Failed to move mouse over search box: ", err)
	}
	testing.ContextLog(ctx, "Click search box")
	if err := ui.LeftClick(searchBox)(ctx); err != nil {
		s.Fatal("Failed to click search box: ", err)
	}

	searchBoxFocused := searchBox.Focused()
	if err := ui.WaitUntilExists(searchBoxFocused); err != nil {
		s.Fatal("Search box is not focused after clicking")
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()
	if err := kb.Type(ctx, "Hello"); err != nil {
		s.Fatal("Failed to enter search text Hello: ", err)
	}

	searchButton := nodewith.Name("Google Search").Role(role.Button)
	if err := ui.LeftClick(searchButton)(ctx); err != nil {
		s.Fatal("Failed to click search botton: ", err)
	}
}
