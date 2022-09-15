// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/baserpc"
	ui "chromiumos/tast/services/cros/ui"
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
			"tast.cros.baserpc.FaillogService",
			"tast.cros.ui.AutomationService",
		},
		Fixture: "chromeOobeWith1BTPeer",
		Timeout: time.Second * 15,
	})
}

// OobeHidMouseOnly tests that a single Blueooth mouse is connected to during OOBE.
func OobeHidMouseOnly(ctx context.Context, s *testing.State) {
	//fv := s.FixtValue().(*bluetooth.FixtValue)

	// Shorten deadline to leave time for cleanup
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(cleanupCtx, 10*time.Second)
	defer cancel()

	rpcClient, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer rpcClient.Close(cleanupCtx)

	uiautoSvc := ui.NewAutomationServiceClient(rpcClient.Conn)
	faillog := baserpc.NewFaillogServiceClient(rpcClient.Conn)

	if response, err := faillog.Create(ctx, &emptypb.Empty{}); err != nil {
		s.Fatal("Failed to dump the UI tree")
	} else {
		testing.ContextLog(ctx, "UI tree dumped to "+response.Path)
	}

	// Verify no keyboard found on device.
	keyboardNotDetectedTextNode := &ui.Finder{
		NodeWiths: []*ui.NodeWith{
			{Value: &ui.NodeWith_HasClass{HasClass: "Widget"}},
		},
	}
	if _, err := uiautoSvc.WaitUntilExists(
		ctx, &ui.WaitUntilExistsRequest{Finder: keyboardNotDetectedTextNode}); err != nil {
		s.Fatal("Failed keyboard detected on DUT: ", err)
	}

	// ui := uiauto.New(bts.tconn)
	// if err := ui.WaitUntilExists(nodewith.Name("continue"))(ctx); err != nil {
	// 	return nil, errors.Wrap(err, "failed to find continue button from service")
	// }

	// _, err = fv.BTS.WaitForCancelButton(ctx, &emptypb.Empty{})
	// if err != nil {
	// 	s.Fatal("Failed to wait for cancel button: ", err)
	// }

	// // Discover btpeer as a mouse.
	// mouseDevice, error := bluetooth.NewEmulatedBTPeerDevice(ctx, fv.BTPeers[0].BluetoothKeyboardDevice())
	// if err != nil {
	// 	s.Fatal("Failed to configure btpeer as a mouse device: ", error)
	// }
	// if mouseDevice.DeviceType() != cbt.DeviceTypeMouse {
	// 	s.Fatalf("Attempted to emulate btpeer device as a %s, but the actual device type is %s", cbt.DeviceTypeMouse, mouseDevice.DeviceType())
	// }

	// _, err = fv.BTS.WaitForCancelButton(ctx, &emptypb.Empty{})
	// if err != nil {
	// 	s.Fatal("Failed to wait for cancel button: ", err)
	// }

	// if _, err := fv.BTS.DiscoverDevice(ctx, &bts.DiscoverDeviceRequest{
	// 	Device: mouseDevice.BTSDevice(),
	// }); err != nil {
	// 	s.Fatalf("DUT failed to discover btpeer as %s: %v", mouseDevice.String(), err)
	// }
}
