// Copyright 2022 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HidScreenShown,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that HID screen is shown on Chromebase, Chromebox and Chromebit form factors",
		Contacts: []string{
			"osamafathy@google.com",
			"chromeos-sw-engprod@google.com",
			"cros-oac@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		VarDeps:      []string{"ui.signinProfileTestExtensionManifestKey"},
		Params: []testing.Param{{
			ExtraHardwareDeps: hwdep.D(hwdep.FormFactor(hwdep.Chromebase, hwdep.Chromebox, hwdep.Chromebit)),
		}},
	})
}

func HidScreenShown(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx,
		chrome.NoLogin(),
		chrome.DontDisableHIDScreenOnOobe(),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(cleanupCtx, time.Second*10)
	defer cancel()
	defer cr.Close(cleanupCtx)

	oobeConn, err := cr.WaitForOOBEConnection(ctx)
	if err != nil {
		s.Fatal("Failed to create OOBE connection: ", err)
	}
	defer oobeConn.Close()

	tconn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create the signin profile test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	ui := uiauto.New(tconn).WithTimeout(10 * time.Second)

	if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.HIDDetectionScreen.isVisible()"); err != nil {
		s.Fatal("Failed to wait for the HID detection screen to be visible: ", err)
	}

	var keyboardDetected bool
	if err := oobeConn.Eval(ctx, "OobeAPI.screens.HIDDetectionScreen.keyboardDetected()", &keyboardDetected); err != nil {
		s.Fatal("Failed to evaluate whether a keyboard is connected: ", err)
	}
	if keyboardDetected {
		s.Fatal("Detected a keyboard while no keyboard is connected")
	}

	var mouseDetected bool
	if err := oobeConn.Eval(ctx, "OobeAPI.screens.HIDDetectionScreen.mouseDetected()", &mouseDetected); err != nil {
		s.Fatal("Failed to evaluate whether a mouse is connected: ", err)
	}
	if mouseDetected {
		s.Fatal("Detected a mouse while no mouse is connected")
	}

	s.Log("Adding a virtual keyboard")
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	if err := oobeConn.Eval(ctx, "OobeAPI.screens.HIDDetectionScreen.keyboardDetected()", &keyboardDetected); err != nil {
		s.Fatal("Failed to evaluate whether a keyboard is connected: ", err)
	}
	if !keyboardDetected {
		s.Fatal("Failed to Detect a keyboard after a virtual keyboard was created: ", err)
	}

	if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.HIDDetectionScreen.canClickNext()"); err != nil {
		s.Fatal("Failed to wait for the continue button to be enabled: ", err)
	}

	var nextButtonName string
	if err := oobeConn.Eval(ctx, "OobeAPI.screens.HIDDetectionScreen.getNextButtonName()", &nextButtonName); err != nil {
		s.Fatal("Failed to get next button name: ", err)
	}
	nextButton := nodewith.Role(role.Button).Name(nextButtonName)
	if err := uiauto.Combine("Click next on the HID detection screen",
		ui.WaitUntilExists(nextButton),
		ui.LeftClickUntil(nextButton, ui.Gone(nextButton)),
	)(ctx); err != nil {
		s.Fatal("Failed to click the HID detection screen next button: ", err)
	}

	if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.WelcomeScreen.isVisible()"); err != nil {
		s.Fatal("Failed to wait for the welcome screen to be visible: ", err)
	}
}
