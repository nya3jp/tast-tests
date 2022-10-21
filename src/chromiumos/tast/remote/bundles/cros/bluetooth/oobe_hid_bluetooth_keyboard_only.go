// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/bluetooth"
	util "chromiumos/tast/remote/bundles/cros/bluetooth/bluetoothutil"
	crui "chromiumos/tast/remote/cros/ui"
	oobeui "chromiumos/tast/remote/cros/ui/oobeui"
	"chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OobeHidBluetoothKeyboardOnly,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that a bluetooth keyboard can be used to complete OOBE",
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

	util.TurnOfServoKeyboardIfOn(ctx, s)

	if err := crui.CheckNodeWithNameExists(ctx, uiautoSvc, oobeui.SearchingForKeyboardNodeName); err != nil {
		s.Fatal("Failed to find node: ", err)
	}

	// Discover btPeer as a keyboard.
	keyboardDevice, err := bluetooth.NewEmulatedBTPeerDevice(ctx, fv.BTPeers[0].BluetoothKeyboardDevice())
	if err != nil {
		s.Fatalf("Failed to configure btpeer as a %s device: %s", keyboardDevice.DeviceType(), err)
	}

	// Verify keyboard device is found.
	// TODO(b/254524000): use approraite authentication method.
	if err := crui.CheckNodeWithNameExists(ctx, uiautoSvc, oobeui.FoundKeyboardNodeName); err != nil {
		s.Fatal("Failed to find node: ", err)
	}

	if result, err := keyboardDevice.RPC().AdapterPowerOff(ctx); err != nil || !result {
		s.Fatal("Failed to turn of btPeer adapter: ", err)
	}

	if err := crui.CheckNodeWithNameExists(ctx, uiautoSvc, oobeui.SearchingForKeyboardNodeName); err != nil {
		s.Fatal("Failed to find node: ", err)
	}

	// Turn on keyboard device and check that keyboard device is paired to.
	if result, err := keyboardDevice.RPC().AdapterPowerOn(ctx); err != nil || !result {
		s.Fatal("Failed to power on btPeer adapter: ", err)
	}

	// Verify keyboard device is found.
	if err := crui.CheckNodeWithNameExists(ctx, uiautoSvc, oobeui.FoundKeyboardNodeName); err != nil {
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

	// Clean up before ending test, this ensures btPeer does not try to pair with DUT
	// after test ends.
	if err := keyboardDevice.RPC().Reset(ctx); err != nil {
		s.Fatal("Failed to reset device: ", err)
	}
}
