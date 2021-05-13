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
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ExtendedDisplayClick,
		Desc:         "A test to debug the click issue on extended display",
		Contacts:     []string{"xliu@cienet.com"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Fixture:      "chromeLoggedIn",
	})
}

func ExtendedDisplayClick(ctx context.Context, s *testing.State) {
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
	testing.ContextLogf(ctx, "Location of window 1: %d", winInfo1.Location)
	winInfo2, err := ui.Info(ctx, window2)
	if err != nil {
		s.Fatal("Failed to get windows 2 info: ", err)
	}
	testing.ContextLogf(ctx, "Location of window 2: %d", winInfo2.Location)

	// Print shelf information.
	shelf := nodewith.ClassName("ShelfView").Name("Shelf")
	shelfInfo1, err := ui.Info(ctx, shelf.Ancestor(window1))
	if err != nil {
		s.Fatal("Failed to get shelf 1 info: ", err)
	}
	testing.ContextLogf(ctx, "Location of shelf 1: %d", shelfInfo1.Location)
	shelfInfo2, err := ui.Info(ctx, shelf.Ancestor(window2))
	if err != nil {
		s.Fatal("Failed to get shelf 2 info: ", err)
	}
	testing.ContextLogf(ctx, "Location of shelf 2: %d", shelfInfo2.Location)

	// Print "Chrome" icon information.
	chromeIcon := nodewith.Name("Google Chrome").ClassName("ash/ShelfAppButton")
	chromeInfo1, err := ui.Info(ctx, chromeIcon.Ancestor(window1))
	if err != nil {
		s.Fatal("Failed to get chrome 1 info: ", err)
	}
	testing.ContextLogf(ctx, "Location of chrome 1: %d", chromeInfo1.Location)
	chromeInfo2, err := ui.Info(ctx, chromeIcon.Ancestor(window2))
	if err != nil {
		s.Fatal("Failed to get chrome 2 info: ", err)
	}
	testing.ContextLogf(ctx, "Location of chrome 2: %d", chromeInfo2.Location)

	// Click the "Chrome" icon on the extended display until Chrome window opens.
	// This is where the error happens: instead of clicking on the "Chrome" icon, the mouse is over the
	// status tray on extended display. Apparently, the coordinates got from node location is translated
	// to screen coordinate differently when issuing the click.
	chromeWindow := nodewith.Role(role.Window).ClassName("BrowserFrame").NameStartingWith("Chrome")
	if err := ui.WithTimeout(30*time.Second).LeftClickUntil(
		chromeIcon.Ancestor(window2),
		ui.WithTimeout(4*time.Second).WaitUntilExists(chromeWindow),
	)(ctx); err != nil {
		s.Fatal("Failed to click chrome icon and launch chrome browser on extended display: ", err)
	}
}
