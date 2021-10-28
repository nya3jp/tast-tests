// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package oobe

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: HidScreenShown,
		Desc: "Smoke test that clicks through OOBE",
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
	cr, err := chrome.New(ctx, chrome.NoLogin())
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	oobeConn, err := cr.WaitForOOBEConnection(ctx)
	if err != nil {
		s.Fatal("Failed to create OOBE connection: ", err)
	}
	defer oobeConn.Close()
	if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.HIDDetectionScreen.isVisible()"); err != nil {
		s.Fatal("Failed to wait for the HID detection screen to be visible: ", err)
	}
	if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.HIDDetectionScreen.emulateDevicesConnected()"); err != nil {
		s.Fatal("Failed to emulate devices connection in HID detection screen: ", err)
	}
	if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.HIDDetectionScreen.isEnabled()"); err != nil {
		s.Fatal("Failed to wait for the HID detection screen next button to be enabled: ", err)
	}
	if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.HIDDetectionScreen.clickNext()"); err != nil {
		s.Fatal("Failed to click HID detection screen next button: ", err)
	}
	if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.WelcomeScreen.isVisible()"); err != nil {
		s.Fatal("Failed to wait for the welcome screen to be visible: ", err)
	}
}
