// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package oobe

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/oobe/fixture"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	oobeHelper "chromiumos/tast/local/oobe"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HidScreenUsbMouseOnly,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that a single usb mouse device can be connected in OOBE HID Detection screen",
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

func HidScreenUsbMouseOnly(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(cleanupCtx, time.Second*10)
	defer cancel()

	cr := s.FixtValue().(*fixture.ChromeOobeHidDetection).Chrome
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
	if err := oobeHelper.IsHidDetectionSearchingForMouse(ctx, oobeConn, tconn); err == nil {
		s.Fatal("Expected mouse device to be found: ", err)
	}

	if err := oobeHelper.IsHidDetectionContinueButtonEnabled(ctx, oobeConn); err != nil {
		s.Fatal("Expected continue button to be enabled: ", err)
	}

	// unplug mouse device.
	mouse.Close()

	// Check that no mouse is detected.
	if err := oobeHelper.IsHidDetectionSearchingForMouse(ctx, oobeConn, tconn); err != nil {
		s.Fatal("Expected mouse device to be disconnected: ", err)
	}

	if err := oobeHelper.IsHidDetectionContinueButtonEnabled(ctx, oobeConn); err == nil {
		s.Fatal("Expected continue button to be disabled: ", err)
	}

	// Reconnect mouse device.
	mouse, err = input.Mouse(ctx)
	if err != nil {
		s.Fatal("Failed to create a virtual mouse: ", err)
	}

	defer mouse.Close()

	// Check that a mouse is detected.
	if err := oobeHelper.IsHidDetectionSearchingForMouse(ctx, oobeConn, tconn); err == nil {
		s.Fatal("Expected mouse device to be found: ", err)
	}

	if err := oobeHelper.IsHidDetectionContinueButtonEnabled(ctx, oobeConn); err != nil {
		s.Fatal("Expected continue button to be enabled: ", err)
	}

	// Click the next button.
	if err := oobeHelper.ClickHidScreenNextButton(ctx, oobeConn, tconn); err != nil {
		s.Fatal("Failed click on next button: ", err)
	}

	// Check that the Welcome screen is visible.
	if err := oobeHelper.IsWelcomeScreenVisible(ctx, oobeConn); err != nil {
		s.Fatal("Failed to wait for the welcome screen to be visible: ", err)
	}
}
