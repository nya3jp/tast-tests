// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package oobe

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: HidScreenShown,
		Desc: "Checks that HID screen is shown on Chromebase, Chromebox and Chromebit form factors",
		Contacts: []string{
			"osamafathy@google.com",
			"chromeos-sw-engprod@google.com",
			"cros-oac@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			ExtraHardwareDeps: hwdep.D(hwdep.FormFactor(hwdep.Chromebase, hwdep.Chromebox, hwdep.Chromebit)),
		}},
	})
}

func HidScreenShown(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.EnableHIDScreen(), chrome.NoLogin())
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	oobeConn, err := cr.WaitForOOBEConnection(ctx)

	defer oobeConn.Close()

	if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.HIDDetectionScreen.isVisible()"); err != nil {
		s.Fatal("Failed to wait for the HID detection screen to be visible: ", err)
	}

	if err := oobeConn.Eval(ctx, "!OobeAPI.screens.HIDDetectionScreen.keyboardDetected()", nil); err != nil {
		s.Log("Detected a keyboard while no keyboard is connected: ", err)
	}

	if err := oobeConn.Eval(ctx, "!OobeAPI.screens.HIDDetectionScreen.mouseDetected()", nil); err != nil {
		s.Log("Detected a mouse while no mouse is connected: ", err)
	}

	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	if err := oobeConn.Eval(ctx, "OobeAPI.screens.HIDDetectionScreen.keyboardDetected()", nil); err != nil {
		s.Log("Failed to detect a keyboard: ", err)
	}

	if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.HIDDetectionScreen.canClickNext()"); err != nil {
		s.Fatal("Failed to wait for keyboard to be detected: ", err)
	}

	if err := oobeConn.Eval(ctx, "OobeAPI.screens.HIDDetectionScreen.clickNext()", nil); err != nil {
		s.Fatal("Failed to click HID detection screen next button: ", err)
	}

	if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.WelcomeScreen.isVisible()"); err != nil {
		s.Fatal("Failed to wait for the welcome screen to be visible: ", err)
	}
}
