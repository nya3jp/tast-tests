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
	"chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OobeHidBluetoothMouseOnly,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that a bluetooth mouse is connected to in OOBE",
		Contacts: []string{
			"tjohnsonkanu@google.com",
			"cros-connectivity@google.com",
		},
		Attr:         []string{"group:mainline", "informational", "group:bluetooth", "bluetooth_btpeers_1"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps: []string{
			"tast.cros.ui.AutomationService",
			"tast.cros.ui.ChromeUIService",
			"tast.cros.bluetooth.BTTestService",
		},
		Fixture:      "chromeOobeWith1BTPeer",
		HardwareDeps: hwdep.D(hwdep.FormFactor(hwdep.Chromebox, hwdep.Chromebit)),
	})
}

func checkNodeWithNameExists(ctx context.Context, uiautoSvc ui.AutomationServiceClient, s *testing.State, name string) {
	finder := &ui.Finder{
		NodeWiths: []*ui.NodeWith{
			{Value: &ui.NodeWith_NameContaining{NameContaining: name}},
			{Value: &ui.NodeWith_First{First: true}},
		},
	}
	if _, err := uiautoSvc.WaitUntilExists(
		ctx, &ui.WaitUntilExistsRequest{Finder: finder}); err != nil {
		s.Fatalf("Failed to find %s: %v", name, err)
	}
}

// OobeHidBluetoothMouseOnly tests that a single Bluetooth mouse is connected to during OOBE.
func OobeHidBluetoothMouseOnly(ctx context.Context, s *testing.State) {
	// TODO(b/246649651): Move these constants to a different file if they are
	// used by other test.
	const SearchingForPointerNodeName = "Searching for pointing device"
	const FoundPointerNodeName = "Pointing device connected"
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

	// Verify pointer device is not found.
	checkNodeWithNameExists(ctx, uiautoSvc, s, SearchingForPointerNodeName)

	// Discover btPeer as a mouse.
	mouseDevice, err := bluetooth.NewEmulatedBTPeerDevice(ctx, fv.BTPeers[0].BluetoothMouseDevice())
	if err != nil {
		s.Fatalf("Failed to configure btpeer as a %s device: %s", mouseDevice.DeviceType(), err)
	}

	if result, err := mouseDevice.RPC().AdapterPowerOn(ctx); err != nil || !result {
		s.Fatal("Failed to power on btPeer adapter: ", err)
	}

	// Verify pointer device is found.
	checkNodeWithNameExists(ctx, uiautoSvc, s, FoundPointerNodeName)

	// Turn off mouse device and check that DUT is searching for mouse.
	if result, err := mouseDevice.RPC().AdapterPowerOff(ctx); err != nil || !result {
		s.Fatal("Failed to turn of btPeer adapter: ", err)
	}

	checkNodeWithNameExists(ctx, uiautoSvc, s, SearchingForPointerNodeName)

	// Turn on mouse device and check that mouse device is paired to.
	if result, err := mouseDevice.RPC().AdapterPowerOn(ctx); err != nil || !result {
		s.Fatal("Failed to power on btPeer adapter: ", err)
	}

	// Verify pointer device is found.
	checkNodeWithNameExists(ctx, uiautoSvc, s, FoundPointerNodeName)

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
	if err := mouseDevice.RPC().Reset(ctx); err != nil {
		s.Fatal("Failed to reset device: ", err)
	}
}
