// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/bluetooth"
	crui "chromiumos/tast/remote/cros/ui"
	"chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OobeHidBluetoothKeyboardOnly,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that a bluetooth keyboard is connected to in OOBE",
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

// OobeHidBluetoothKeyboardOnly tests that a single Blueooth mouse is connected to during OOBE.
func OobeHidBluetoothKeyboardOnly(ctx context.Context, s *testing.State) {

	const UsbKeyboardConnectedNode = "USB keyboard connected"
	const SearchingForKeyboardNodeName = "Searching for keyboard"
	const FoundKeyboardNodeName = "KEYBD_REF"

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
		if _, err := crUISvc.DumpUITree(ctx, &emptypb.Empty{}); err != nil {
			testing.ContextLog(ctx, "Failed to dump UI tree: ", err)
		}
	}()

	// check if a usb keyboard is connected, if it  is turn of servo device.
	if err := crui.CheckNodeWithNameExists(ctx, uiautoSvc, UsbKeyboardConnectedNode); err == nil {
		// Set up Servo in remote tests.
		dut := s.DUT()
		pxy, err := servo.NewProxy(ctx, s.RequiredVar("servo"), dut.KeyFile(), dut.KeyDir())
		if err != nil {
			s.Fatal("Failed to connect to servo: ", err)
		}
		defer pxy.Close(ctx)

		if err := pxy.Servo().SetOnOff(ctx, servo.USBKeyboard, servo.Off); err != nil {
			s.Fatal("Failed to turn of servo: ", err)
		}
	}

	if err := crui.CheckNodeWithNameExists(ctx, uiautoSvc, SearchingForKeyboardNodeName); err != nil {
		s.Fatal("Failed to find node: ", err)
	}

	// Discover btPeer as a keyboard.
	keyboardDevice, err := bluetooth.NewEmulatedBTPeerDevice(ctx, fv.BTPeers[0].BluetoothKeyboardDevice())
	if err != nil {
		s.Fatalf("Failed to configure btpeer as a %s device: %s", keyboardDevice.DeviceType(), err)
	}

	// Verify keyboard device is found.
	if err := crui.CheckNodeWithNameExists(ctx, uiautoSvc, FoundKeyboardNodeName); err != nil {
		s.Fatal("Failed to find node: ", err)
	}

	if result, err := keyboardDevice.RPC().AdapterPowerOff(ctx); err != nil || !result {
		s.Fatal("Failed to turn of btPeer adapter: ", err)
	}

	if err := crui.CheckNodeWithNameExists(ctx, uiautoSvc, SearchingForKeyboardNodeName); err != nil {
		s.Fatal("Failed to find node: ", err)
	}

	// Turn on keyboard device and check that keyboard device is paired to.
	if result, err := keyboardDevice.RPC().AdapterPowerOn(ctx); err != nil || !result {
		s.Fatal("Failed to power on btPeer adapter: ", err)
	}

	// Verify keyboard device is found.
	if err := crui.CheckNodeWithNameExists(ctx, uiautoSvc, FoundKeyboardNodeName); err != nil {
		s.Fatal("Failed to find node: ", err)
	}

	// Navigate to welcome screen.
	continueButtonFinder := &ui.Finder{
		NodeWiths: []*ui.NodeWith{
			{Value: &ui.NodeWith_Name{Name: "Continue"}},
			{Value: &ui.NodeWith_Role{Role: ui.Role_ROLE_BUTTON}},
		},
	}
	if _, err := uiautoSvc.WaitUntilExists(
		ctx, &ui.WaitUntilExistsRequest{Finder: continueButtonFinder}); err != nil {
		s.Fatal("Failed to find continue button: ", err)
	}

	if _, err := uiautoSvc.LeftClick(
		ctx, &ui.LeftClickRequest{Finder: continueButtonFinder}); err != nil {
		s.Fatal("Failed to click continue button: ", err)
	}

	if _, err := crUISvc.WaitForWelcomeScreen(ctx, &emptypb.Empty{}); err != nil {
		s.Fatal("Failed to enter welcome page")
	}

	// Clean up before ending test, this ensures btPeer does not try to pair with DUT
	// after test ends.
	if err := keyboardDevice.RPC().Reset(ctx); err != nil {
		s.Fatal("Failed to reset device: ", err)
	}
}
