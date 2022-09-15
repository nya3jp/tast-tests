// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"

	"chromiumos/tast/common/chameleon"
	cbt "chromiumos/tast/common/chameleon/devices/common/bluetooth"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/bluetooth"
	"chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OobeHidMouseOnly,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that a bluetooth mouse is connected to in OOBE",
		Contacts: []string{
			"tjohnsonkanu@google.com",
			"cros-connectivity@google.com",
		},
		Attr:         []string{},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps: []string{
			"tast.cros.ui.AutomationService",
			"tast.cros.ui.ChromeUIService",
			"tast.cros.bluetooth.BTTestService",
		},
		Fixture: "chromeOobeWith1BTPeer",
		Timeout: time.Second * 60,
	})
}

// OobeHidMouseOnly tests that a single Blueooth mouse is connected to during OOBE.
func OobeHidMouseOnly(ctx context.Context, s *testing.State) {
	fv := s.FixtValue().(*bluetooth.FixtValue)

	// Shorten deadline to leave time for cleanup
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(cleanupCtx, 5*time.Second)
	defer cancel()

	uiautoSvc := ui.NewAutomationServiceClient(fv.DUTRPCClient.Conn)
	crUISvc := ui.NewChromeUIServiceClient(fv.DUTRPCClient.Conn)

	// Disconnet all connected bluetooth devices.
	if _, err := fv.BTS.DisconnectAllDevices(ctx, &emptypb.Empty{}); err != nil {
		s.Fatal("Failed to disconnect all devices: ", err)
	}

	// Verify no mouse found on device.
	findNode(ctx, uiautoSvc, s, "Searching for pointing device")

	// Discover btpeer as a mouse.
	emulateMouseDevice(ctx, fv.BTPeers[0], s)
	testing.Sleep(ctx, 10*time.Second)

	if _, err := crUISvc.DumpUITree(ctx, &emptypb.Empty{}); err != nil {
		s.Fatal("Failed to dump the UI tree")
	}

	// Verify mouse device is found.
	findNode(ctx, uiautoSvc, s, "Bluetooth mouse paired")

	// Turn off mouse device and check that DUT is searching for mouse.
	if err := fv.BTPeers[0].BluetoothMouseDevice().Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot device: ", err)
	}

	testing.Sleep(ctx, 10*time.Second)

	findNode(ctx, uiautoSvc, s, "Searching for pointing device")

	emulateMouseDevice(ctx, fv.BTPeers[0], s)
	testing.Sleep(ctx, 10*time.Second)

	// Verify mouse device is found.
	findNode(ctx, uiautoSvc, s, "Bluetooth mouse paired")

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
		s.Fatal("Failed to click feedback button: ", err)
	}

	testing.Sleep(ctx, 5*time.Second)

	if _, err := crUISvc.WaitForWelcomeScreen(ctx, &emptypb.Empty{}); err != nil {
		s.Fatal("Failed to enter welcome page")
	}
}

func emulateMouseDevice(ctx context.Context, btPeer chameleon.Chameleond, s *testing.State) {
	mouseDevice, err := bluetooth.NewEmulatedBTPeerDevice(ctx, btPeer.BluetoothMouseDevice())
	if err != nil {
		s.Fatal("Failed to configure btpeer as a mouse device: ", err)
	}
	if mouseDevice.DeviceType() != cbt.DeviceTypeMouse {
		s.Fatalf("Attempted to emulate btpeer device as a %s, but the actual device type is %s", cbt.DeviceTypeMouse, mouseDevice.DeviceType())
	}
}

func findNode(ctx context.Context, uiautoSvc ui.AutomationServiceClient, s *testing.State, name string) {
	searchingForPointerNode := &ui.Finder{
		NodeWiths: []*ui.NodeWith{
			{Value: &ui.NodeWith_NameContaining{NameContaining: name}},
			{Value: &ui.NodeWith_First{First: true}},
		},
	}
	if _, err := uiautoSvc.WaitUntilExists(
		ctx, &ui.WaitUntilExistsRequest{Finder: searchingForPointerNode}); err != nil {
		s.Fatalf("Failed to find %s: %v", name, err)
	}
}
