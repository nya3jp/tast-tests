// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package oobe

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CloudGamingMarketingOptIn,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test that we show gaming-specific marketing opt in screen on a cloud gaming board",
		Contacts: []string{
			"bohdanty@google.com",
			"chromeos-sw-engprod@google.com",
			"cros-oac@google.com",
			"cros-oobe@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      chrome.LoginTimeout + 3*time.Minute,
		// This test should run only on gaming models.
		HardwareDeps: hwdep.D(hwdep.Model("taniks", "osiris")),
	})
}

func CloudGamingMarketingOptIn(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cr, err := chrome.New(ctx, chrome.DontSkipOOBEAfterLogin())
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	oobeConn, err := cr.WaitForOOBEConnection(ctx)
	if err != nil {
		s.Fatal("Failed to create OOBE connection: ", err)
	}
	defer oobeConn.Close()

	if err := oobeConn.Eval(ctx, "OobeAPI.advanceToScreen('marketing-opt-in')", nil); err != nil {
		s.Fatal("Failed to advance to the marketing opt-in screen: ", err)
	}

	if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.MarketingOptInScreen.isVisible()"); err != nil {
		s.Fatal("Failed to wait for the marketing opt-in screen: ", err)
	}

	var gamingTitleVisible bool
	if err := oobeConn.Eval(ctx, "OobeAPI.screens.MarketingOptInScreen.isMarketingOptInGameDeviceTitleVisible()", &gamingTitleVisible); err != nil {
		s.Fatal("Failed to fetch visibility of gaming-specific titile: ", err)
	}

	if !gamingTitleVisible {
		s.Fatal("Gaming-specific title should be shown on the markting opt-in for a gaming models")
	}

	ui := uiauto.New(tconn).WithTimeout(60 * time.Second)

	var getStartedButtonName string
	if err := oobeConn.Eval(ctx, "OobeAPI.screens.MarketingOptInScreen.getGetStartedButtonName()", &getStartedButtonName); err != nil {
		s.Fatal("Failed to retrieve the get started button name: ", err)
	}

	getStartedButton := nodewith.Name(getStartedButtonName).Role(role.Button)
	if err := ui.WaitUntilExists(getStartedButton)(ctx); err != nil {
		s.Fatal("Failed to wait until get started button is shown: ", err)
	}
	if err := ui.LeftClick(getStartedButton)(ctx); err != nil {
		s.Fatal("Failed to click on get started button: ", err)
	}
}
