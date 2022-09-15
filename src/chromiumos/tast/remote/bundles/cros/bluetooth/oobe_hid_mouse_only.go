// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"

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
		Timeout: time.Second * 15,
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

	// Verify no keyboard found on device.
	keyboardNotDetectedTextNode := &ui.Finder{
		NodeWiths: []*ui.NodeWith{
			{Value: &ui.NodeWith_NameContaining{NameContaining: "Searching for pointing device"}},
			{Value: &ui.NodeWith_First{First: true}},
		},
	}
	if _, err := uiautoSvc.WaitUntilExists(
		ctx, &ui.WaitUntilExistsRequest{Finder: keyboardNotDetectedTextNode}); err != nil {
		s.Fatal("Failed keyboard detected on DUT: ", err)
	}

	// Discover btpeer as a mouse.
	mouseDevice, err := bluetooth.NewEmulatedBTPeerDevice(ctx, fv.BTPeers[0].BluetoothMouseDevice())
	if err != nil {
		s.Fatal("Failed to configure btpeer as a mouse device: ", err)
	}
	if mouseDevice.DeviceType() != cbt.DeviceTypeMouse {
		s.Fatalf("Attempted to emulate btpeer device as a %s, but the actual device type is %s", cbt.DeviceTypeMouse, mouseDevice.DeviceType())
	}

	// if _, err := fv.BTS.DiscoverDevice(ctx, &bts.DiscoverDeviceRequest{
	// 	Device: mouseDevice.BTSDevice(),
	// }); err != nil {
	// 	s.Fatalf("DUT failed to discover btpeer1 as %s: %v", mouseDevice.String(), err)
	// }

	if _, err := crUISvc.DumpUITree(ctx, &emptypb.Empty{}); err != nil {
		s.Fatal("Failed to dump the UI tree")
	}
}
