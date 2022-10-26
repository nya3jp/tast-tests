// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package oobe

import (
	"context"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/bluetooth"
	crui "chromiumos/tast/remote/cros/ui"
	oobeui "chromiumos/tast/remote/cros/ui/oobeui"
	"chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HidUsbKeyboardOnly,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that a USB keyboard is connected to in OOBE",
		Contacts: []string{
			"tjohnsonkanu@google.com",
			"cros-connectivity@google.com",
		},
		VarDeps:      []string{"servo"},
		Attr:         []string{"group:mainline", "informational", "group:bluetooth", "bluetooth_btpeers_1"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps: []string{
			"tast.cros.ui.AutomationService",
			"tast.cros.ui.ChromeUIService",
			"tast.cros.bluetooth.BTTestService",
		},
		Fixture:      "chromeOobeWith1BTPeer",
		HardwareDeps: hwdep.D(hwdep.FormFactor(hwdep.Chromebase, hwdep.Chromebox, hwdep.Chromebit)),
	})
}

// HidUsbKeyboardOnly tests that a single USB keyboard is connected to during OOBE.
func HidUsbKeyboardOnly(ctx context.Context, s *testing.State) {
	fv := s.FixtValue().(*bluetooth.FixtValue)

	// Shorten deadline to leave time for cleanup
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(cleanupCtx, 5*time.Second)
	defer cancel()

	uiautoSvc := ui.NewAutomationServiceClient(fv.DUTRPCClient.Conn)
	crUISvc := ui.NewChromeUIServiceClient(fv.DUTRPCClient.Conn)

	defer func() {
		if !s.HasError() {
			return
		}
		if _, err := crUISvc.DumpUITree(cleanupCtx, &emptypb.Empty{}); err != nil {
			testing.ContextLog(cleanupCtx, "Failed to dump UI tree: ", err)
		}
	}()

	// Set up Servo in remote tests.
	dut := s.DUT()
	pxy, err := servo.NewProxy(ctx, s.RequiredVar("servo"), dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	// Cleanup servo.
	defer pxy.Close(ctx)

	// Make sure USB keyboard is connected.
	isOn, err := pxy.Servo().GetOnOff(ctx, servo.USBKeyboard)
	if err != nil {
		s.Fatal("Failed to get servo keyboard status: ", err)
	}

	// If USB keyboard is off turn it on.
	if !isOn {
		if err := pxy.Servo().SetOnOff(ctx, servo.USBKeyboard, servo.On); err != nil {
			s.Fatal("Failed to turn on servo: ", err)
		}
	}

	// Verify mouse device is found.
	if err := crui.CheckNodeWithNameExists(ctx, uiautoSvc, oobeui.FoundUsbKeyboardNodeName); err != nil {
		s.Fatal("Failed to find node: ", err)
	}

	// Turn off USB keyboard.
	if err := pxy.Servo().SetOnOff(ctx, servo.USBKeyboard, servo.Off); err != nil {
		s.Fatal("Failed to turn off servo: ", err)
	}

	// Verify USB keyboard is not found.
	if err := crui.CheckNodeWithNameExists(ctx, uiautoSvc, oobeui.SearchingForKeyboardNodeName); err != nil {
		s.Fatal("Failed to find node: ", err)
	}

	// Turn on USB keyboard.
	if err := pxy.Servo().SetOnOff(ctx, servo.USBKeyboard, servo.On); err != nil {
		s.Fatal("Failed to turn on servo: ", err)
	}

	// Verify USB keyboard is not found.
	if err := crui.CheckNodeWithNameExists(ctx, uiautoSvc, oobeui.FoundUsbKeyboardNodeName); err != nil {
		s.Fatal("Failed to find node: ", err)
	}

	// Navigate to welcome screen.
	if _, err := uiautoSvc.LeftClick(
		ctx, &ui.LeftClickRequest{Finder: oobeui.ContinueButtonFinder}); err != nil {
		s.Fatal("Failed to click continue button: ", err)
	}

	if _, err := crUISvc.WaitForWelcomeScreen(ctx, &emptypb.Empty{}); err != nil {
		s.Fatal("Failed to enter welcome page")
	}
}
