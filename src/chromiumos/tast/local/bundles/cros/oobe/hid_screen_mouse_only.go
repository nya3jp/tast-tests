// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package oobe

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/oobe"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HidScreenMouseOnly,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that a single mouse device can be connected in OOBE HID Detection screen ",
		Contacts: []string{
			"tjohnsonkanu@google.com",
			"cros-connectivity@google.com",
		},
		VarDeps: []string{
			"ui.signinProfileTestExtensionManifestKey",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.FormFactor(hwdep.Chromebase, hwdep.Chromebox, hwdep.Chromebit)),
		Fixture:      "chromeEnterOobeHidDetection",
		Timeout:      time.Second * 30,
	})
}

func HidScreenMouseOnly(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(cleanupCtx, time.Second*10)
	defer cancel()

	cr := s.FixtValue().(*oobe.ChromeOobeHidDetection).Chrome
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

	// Check that the HID detection screen is visible.
	if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.HIDDetectionScreen.isVisible()"); err != nil {
		s.Fatal("Failed to wait for the HID detection screen to be visible: ", err)
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

	// unplug mouse device.
	mouse.Close()

	// Check that no mouse is detected.
	var mouseNotDetectedText string
	if err := oobeConn.Eval(ctx, "OobeAPI.screens.HIDDetectionScreen.getMouseNotDetectedText()", &mouseNotDetectedText); err != nil {
		s.Fatal("Failed to get the text to be shown when no mouse is detected: ", err)
	}
	mouseNotDetectedTextNode := nodewith.Role(role.StaticText).Name(mouseNotDetectedText)
	if err := ui.WaitUntilExists(mouseNotDetectedTextNode)(ctx); err != nil {
		s.Fatal("Failed to find the text indicating that no mouse is detected: ", err)
	}

	// Reconnect mouse device.
	mouse, err = input.Mouse(ctx)
	if err != nil {
		s.Fatal("Failed to create a virtual mouse: ", err)
	}

	// Check that a mouse is detected.
	mousedDetectedTextNode = nodewith.Role(role.StaticText).Name(mousedDetectedText)
	if err := ui.WaitUntilExists(mousedDetectedTextNode)(ctx); err != nil {
		s.Fatal("Failed to find the text indicating that a mouse is connected: ", err)
	}

	// Click the next button.
	if err := oobe.ClickHidScreenNextButton(ctx, ui, oobeConn); err != nil {
		s.Fatal("Failed click on next button: ", err)
	}

	// Check that the Welcome screen is visible.
	if err := oobe.IsWelcomeScreenVisible(ctx, oobeConn); err != nil {
		s.Fatal("Failed to wait for the welcome screen to be visible: ", err)
	}
}
