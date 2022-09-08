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
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MarketingOptIn,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test marketing opt-in screen in tablet and laptop modes",
		Contacts: []string{
			"bohdanty@google.com",
			"chromeos-sw-engprod@google.com",
			"cros-oac@google.com",
			"cros-oobe@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      2*chrome.LoginTimeout + 3*time.Minute,
	})
}

func MarketingOptIn(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 20*time.Second)
	defer cancel()

	// Tablet mode flow.
	func() {
		cr, err := chrome.New(ctx,
			chrome.DontSkipOOBEAfterLogin(),
			chrome.ExtraArgs("--force-tablet-mode=touch_view"),
		)
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

		// Accessibility button will be shown on marketing opt-in only if we
		// triggered GestureNavigationScreen::OnUserAction with
		// kUserActionExitPressed, so we need to set this flag prior to
		// loading the Marketing Opt-In screen. The reason to set flag and
		// don't pass gesture flow manually is to reduce potential flakiness
		// and consume less resources. Gesture flow is already tested with a
		// similar environment in the oobe.GestureNavigation test.
		if err := oobeConn.Eval(ctx, "OobeAPI.setGestureNavigationWasShown(true)", nil); err != nil {
			s.Fatal("Failed to make accessibility button visible: ", err)
		}

		if err := oobeConn.Eval(ctx, "OobeAPI.advanceToScreen('marketing-opt-in')", nil); err != nil {
			s.Fatal("Failed to advance to the marketing opt-in screen: ", err)
		}

		if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.MarketingOptInScreen.isVisible()"); err != nil {
			s.Fatal("Failed to wait for the marketing opt-in screen: ", err)
		}

		ui := uiauto.New(tconn).WithTimeout(60 * time.Second)

		if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.MarketingOptInScreen.isAccessibilityButtonVisible()"); err != nil {
			s.Fatal("Failed to wait until accessibility button is shown: ", err)
		}

		var a11yButtonName string
		if err := oobeConn.Eval(ctx, "OobeAPI.screens.MarketingOptInScreen.getAccessibilityButtonName()", &a11yButtonName); err != nil {
			s.Fatal("Failed to retrieve the accessibility button name: ", err)
		}

		a11yButton := nodewith.Name(a11yButtonName).Role(role.Button)
		if err := ui.WaitUntilExists(a11yButton)(ctx); err != nil {
			s.Fatal("Failed to wait until accessibility button is shown: ", err)
		}
		if err := ui.LeftClick(a11yButton)(ctx); err != nil {
			s.Fatal("Failed to click on accessibility button: ", err)
		}

		if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.MarketingOptInScreen.isAccessibilityStepReadyForTesting()"); err != nil {
			s.Fatal("Failed to wait until accessibility step is shown: ", err)
		}

		var a11yToggleStatus bool
		if err := oobeConn.Eval(ctx, "OobeAPI.screens.MarketingOptInScreen.isAccessibilityToggleOn()", &a11yToggleStatus); err != nil {
			s.Fatal("Failed to wait fetch accessibility toggle status: ", err)
		}

		if a11yToggleStatus {
			s.Fatal("Accessibility toggle should be turned off by default")
		}

		var a11yDoneButtonName string
		if err := oobeConn.Eval(ctx, "OobeAPI.screens.MarketingOptInScreen.getAccessibilityDoneButtonName()", &a11yDoneButtonName); err != nil {
			s.Fatal("Failed to retrieve the accessibility done button name: ", err)
		}

		a11yDoneButton := nodewith.Name(a11yDoneButtonName).Role(role.Button)
		if err := ui.WaitUntilExists(a11yButton)(ctx); err != nil {
			s.Fatal("Failed to wait until accessibility done button is shown: ", err)
		}
		if err := ui.LeftClick(a11yDoneButton)(ctx); err != nil {
			s.Fatal("Failed to click on accessibility done button: ", err)
		}
	}()

	// Laptop flow
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

	ui := uiauto.New(tconn).WithTimeout(60 * time.Second)

	var a11ButtonVisible bool
	if err := oobeConn.Eval(ctx, "OobeAPI.screens.MarketingOptInScreen.isAccessibilityButtonVisible()", &a11ButtonVisible); err != nil {
		s.Fatal("Failed to fetch accessibility button visibility: ", err)
	}

	if a11ButtonVisible {
		s.Fatal("Accessibility button should not be visible in the laptop mode")
	}

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
