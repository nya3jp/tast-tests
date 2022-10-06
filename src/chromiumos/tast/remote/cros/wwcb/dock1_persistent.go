// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// generate .bp.go file command example:
// ~/trunk/src/platform/tast/tools/go.sh generate chromiumos/tast/services/cros/wwcb
// excute command example:
// tast run -var=DockingID=1912901 -var=ExtDispID=2109002 192.168.1.164 wwcb.Dock1Persistent

package wwcb

import (
	"chromiumos/tast/remote/bundles/cros/wwcb/utils"
	"chromiumos/tast/services/cros/wwcb"

	"chromiumos/tast/rpc"
	"chromiumos/tast/testing"

	"context"

	"github.com/golang/protobuf/ptypes/empty"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Dock1Persistent,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "WWCB dock1 test case",
		Contacts:     []string{"nya@chromium.org", "tast-owners@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		Vars:         []string{"DockingID", "ExtDispID"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.wwcb.WWCBService"},
	})
}

func Dock1Persistent(ctx context.Context, s *testing.State) {

	// Init
	ippowerPort := []int{1}
	utils.OpenIppower(ctx, ippowerPort)

	dockingID := s.RequiredVar("DockingID")
	extDispID := s.RequiredVar("ExtDispID")

	utils.InitFixture(ctx)

	// Step1: Connect to the gRPC server on the DUT.
	testing.ContextLog(ctx, "Step1: Connect to the gRPC server on the DUT \n")

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	cr := wwcb.NewWWCBServiceClient(cl.Conn)

	if _, err := cr.New(ctx, &empty.Empty{}); err != nil {
		s.Fatal("init. new", err)
	}
	defer cr.Close(ctx, &empty.Empty{})

	// Step2: turn ExtDisplay 'on'
	testing.ContextLog(ctx, "Step2: turn ExtDisplay 'on' \n")

	utils.ControlFixture(ctx, extDispID, "on")

	// Step3: turn Docking 'on' , check connect docking station to chromebook
	testing.ContextLog(ctx, "Step3: turn Docking 'on' , check connect docking station to chromebook \n")

	utils.ControlFixture(ctx, dockingID, "on")
	if _, err := cr.Dock1PersistentStep3(ctx, &empty.Empty{}); err != nil {
		s.Fatal("3. check connect docking station to chromebook fail", err)
	}

	// Step4: open two apps on external display
	testing.ContextLog(ctx, "Step4: open two apps on external display \n")

	if _, err := cr.Dock1PersistentStep4(ctx, &empty.Empty{}); err != nil {
		s.Fatal("4. open two apps on external display", err)
	}

	// Step5: turn ExtDisplay "off" , VerifyAllWindowsOnDisplay , turn ExtDisplay "on" , VerifyAllWindowsOnDisplay
	testing.ContextLog(ctx, "Step5: turn ExtDisplay 'off' , VerifyAllWindowsOnDisplay , turn ExtDisplay 'on' , VerifyAllWindowsOnDisplay \n")

	utils.ControlFixture(ctx, extDispID, "off")
	req := &wwcb.BoolRequest{SendBool: false}
	if _, err := cr.VerifyAllWindowsOnDisplay(ctx, req); err != nil {
		s.Fatal("5. VerifyAllWindowsOnDisplay off fail", err)
	}
	utils.ControlFixture(ctx, extDispID, "on")

	req = &wwcb.BoolRequest{SendBool: true}
	if _, err := cr.VerifyAllWindowsOnDisplay(ctx, req); err != nil {
		s.Fatal("5. VerifyAllWindowsOnDisplay on fail", err)
	}

	// Step6: test primary mode
	testing.ContextLog(ctx, "Step6: test primary mode \n")

	if _, err := cr.Dock1PersistentStep6(ctx, &empty.Empty{}); err != nil {
		s.Fatal("6. test primary mode fail", err)
	}

	// Step7: turn ExtDisplay 'off' , VerifyAllWindowsOnDisplay , turn ExtDisplay 'on' , VerifyAllWindowsOnDisplay
	testing.ContextLog(ctx, "Step7: turn ExtDisplay 'off' , VerifyAllWindowsOnDisplay , turn ExtDisplay 'on' , VerifyAllWindowsOnDisplay \n")

	utils.ControlFixture(ctx, extDispID, "off")
	req = &wwcb.BoolRequest{SendBool: false}
	if _, err := cr.VerifyAllWindowsOnDisplay(ctx, req); err != nil {
		s.Fatal("7. VerifyAllWindowsOnDisplay off fail", err)
	}
	utils.ControlFixture(ctx, extDispID, "on")

	req = &wwcb.BoolRequest{SendBool: true}
	if _, err := cr.VerifyAllWindowsOnDisplay(ctx, req); err != nil {
		s.Fatal("7. VerifyAllWindowsOnDisplay on fail", err)
	}

	// Step8: test mirror mode
	testing.ContextLog(ctx, "Step8: test mirror mode \n")

	if _, err := cr.Dock1PersistentStep8(ctx, &empty.Empty{}); err != nil {
		s.Fatal("8. test mirror mode fail", err)
	}

	// tear dowin
	utils.CloseAllFixture(ctx)
	utils.CloseIppower(ctx, ippowerPort)
}
