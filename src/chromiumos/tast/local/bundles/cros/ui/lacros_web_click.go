// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LacrosWebClick,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "A test to debug the click issue on extended display web page",
		Contacts:     []string{"xliu@cienet.com"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Fixture:      "lacrosPrimary",
	})
}

func LacrosWebClick(ctx context.Context, s *testing.State) {
	f := s.FixtValue().(lacrosfixt.FixtValue)
	cr := f.Chrome()
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	defer faillog.SaveScreenshotOnError(ctx, cr, s.OutDir(), s.HasError)

	ui := uiauto.New(tconn)
	// Root window on internal display.
	window := nodewith.ClassName("RootWindow-0").Role(role.Window)
	winInfo, err := ui.Info(ctx, window)
	if err != nil {
		s.Fatal("Failed to get windows info: ", err)
	}
	testing.ContextLog(ctx, "Location of window: ", winInfo.Location)

	_, _, cs, err := lacros.Setup(ctx, f, browser.TypeLacros)
	if err != nil {
		s.Fatal("Failed to set up lacros test: ", err)
	}
	// defer lacros.CloseLacros(ctx, l)

	testing.ContextLog(ctx, "Navigate to YT music")
	// NOTE: test will pass if you replace the URL with this one:
	// https://music.youtube.com/playlist?list=RDCLAK5uy_nZiG9ehz_MQoWQxY5yElsLHCcG0tv9PRg
	_, err = cs.NewConn(ctx, "https://music.youtube.com/channel/UCPC0L1d253x-KuMNwa05TpA")
	if err != nil {
		s.Fatal("Failed to open new tab: ", err)
	}

	browser := nodewith.ClassName("BrowserFrame").Ancestor(window)
	browserInfo, err := ui.Info(ctx, browser)
	if err != nil {
		s.Fatal("Failed to get browser info: ", err)
	}
	testing.ContextLog(ctx, "Location of browser: ", browserInfo.Location)

	shuffleButton := nodewith.Name("Shuffle").Role(role.Button).Ancestor(browser)
	shuffleBtnInfo, err := ui.Info(ctx, shuffleButton)
	if err != nil {
		s.Fatal("Failed to get shuffle button info: ", err)
	}
	testing.ContextLog(ctx, "Location of shuffle button: ", shuffleBtnInfo.Location)

	testing.ContextLog(ctx, "Move mouse over to shuffle button")
	if err := ui.MouseMoveTo(shuffleButton, 3*time.Second)(ctx); err != nil {
		s.Fatal("Failed to move mouse over shuffle button: ", err)
	}
	testing.ContextLog(ctx, "Click shuffle button")
	if err := ui.LeftClick(shuffleButton)(ctx); err != nil {
		s.Fatal("Failed to click shuffle button: ", err)
	}

	pauseButton := nodewith.Name("Pause").Role(role.Button).Ancestor(browser)
	if err := ui.WaitUntilExists(pauseButton)(ctx); err != nil {
		s.Fatal("Music is not played after clicking Shuffle button - there is no music Pause button ; probabley the Shuffle button was not clicked")
	}
}
