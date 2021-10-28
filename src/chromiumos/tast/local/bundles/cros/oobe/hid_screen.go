// Copyright 2022 The ChromiumOS Authors
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
		Func:         HidScreen,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that HID screen is shown on Chromebase, Chromebox and Chromebit form factors and skipped on other form factors",
		Contacts: []string{
			"osamafathy@google.com",
			"chromeos-sw-engprod@google.com",
			"cros-oac@google.com",
			"cros-oobe@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		VarDeps: []string{
			"ui.signinProfileTestExtensionManifestKey",
		},
		Timeout: 3 * time.Minute,

		Params: []testing.Param{
			{
				Name:              "shown",
				Val:               true,
				ExtraHardwareDeps: hwdep.D(hwdep.FormFactor(hwdep.Chromebase, hwdep.Chromebox, hwdep.Chromebit)),
			},
			{
				Name:              "skipped",
				Val:               false,
				ExtraHardwareDeps: hwdep.D(hwdep.SkipOnFormFactor(hwdep.Chromebase, hwdep.Chromebox, hwdep.Chromebit)),
			},
		}},
	)
}

func HidScreen(ctx context.Context, s *testing.State) {
	hidShown := s.Param().(bool)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(cleanupCtx, time.Second*10)
	defer cancel()

	cr, err := chrome.New(ctx,
		chrome.NoLogin(),
		chrome.EnableHIDScreenOnOOBE(),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
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

	if hidShown {
		// Check that the HID detection screen is visible.
		if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.HIDDetectionScreen.isVisible()"); err != nil {
			s.Fatal("Failed to wait for the HID detection screen to be visible: ", err)
		}

		// Check that no mouse is detected.
		var mouseNotDetectedText string
		if err := oobeConn.Eval(ctx, "OobeAPI.screens.HIDDetectionScreen.getMouseNotDetectedText()", &mouseNotDetectedText); err != nil {
			s.Fatal("Failed to get the text to be shown when no mouse is detected: ", err)
		}
		mouseNotDetectedTextNode := nodewith.Role(role.StaticText).Name(mouseNotDetectedText)
		if err := ui.WaitUntilExists(mouseNotDetectedTextNode)(ctx); err != nil {
			s.Fatal("Failed to find the text indicating that no mouse is detected: ", err)
		}

		// Check that no keyboard is detected.
		var keyboardNotDetectedText string
		if err := oobeConn.Eval(ctx, "OobeAPI.screens.HIDDetectionScreen.getKeyboardNotDetectedText()", &keyboardNotDetectedText); err != nil {
			s.Fatal("Failed to get the text to be shown when no keyboard is detected: ", err)
		}
		keyboardNotDetectedTextNode := nodewith.Role(role.StaticText).Name(keyboardNotDetectedText)
		if err := ui.WaitUntilExists(keyboardNotDetectedTextNode)(ctx); err != nil {
			s.Fatal("Failed to find the text indicating that no keyboard is detected: ", err)
		}

		// Create a virtual mouse.
		mouse, err := input.Mouse(ctx)
		if err != nil {
			s.Fatal("Failed to create a virtual mouse: ", err)
		}
		defer mouse.Close()

		// Check that a mouse is detected.
		var mousedDetectedText string
		if err := oobeConn.Eval(ctx, "OobeAPI.screens.HIDDetectionScreen.getPointingDeviceDetectedText()", &mousedDetectedText); err != nil {
			s.Fatal("Failed to get the text to be shown when a mouse is detected: ", err)
		}
		mousedDetectedTextNode := nodewith.Role(role.StaticText).Name(mousedDetectedText)
		if err := ui.WaitUntilExists(mousedDetectedTextNode)(ctx); err != nil {
			s.Fatal("Failed to find the text indicating that a mouse is connected: ", err)
		}

		// Create a virtual keyboard.
		keyboard, err := input.Keyboard(ctx)
		if err != nil {
			s.Fatal("Failed to create a virtual keyboard: ", err)
		}
		defer keyboard.Close()

		// Check that a keyboard is detected.
		var keyboardDetectedText string
		if err := oobeConn.Eval(ctx, "OobeAPI.screens.HIDDetectionScreen.getUsbKeyboardDetectedText()", &keyboardDetectedText); err != nil {
			s.Fatal("Failed to get the text to be shown when a keyboard is detected: ", err)
		}
		keyboardDetectedTextNode := nodewith.Role(role.StaticText).Name(keyboardDetectedText)
		if err := ui.WaitUntilExists(keyboardDetectedTextNode)(ctx); err != nil {
			s.Fatal("Failed to find the text indicating that a keyboard is connected: ", err)
		}

		// Click the next button.
		var nextButtonName string
		if err := oobeConn.Eval(ctx, "OobeAPI.screens.HIDDetectionScreen.getNextButtonName()", &nextButtonName); err != nil {
			s.Fatal("Failed to get next button name: ", err)
		}
		nextButton := nodewith.Role(role.Button).Name(nextButtonName)
		if err := uiauto.Combine("Click next button",
			ui.WaitUntilEnabled(nextButton),
			ui.LeftClick(nextButton),
			ui.WaitUntilGone(nextButton),
		)(ctx); err != nil {
			s.Fatal("Failed to click next button: ", err)
		}
	}

	// Check that the Welcome screen is visible.
	if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.WelcomeScreen.isVisible()"); err != nil {
		s.Fatal("Failed to wait for the welcome screen to be visible: ", err)
	}
}
