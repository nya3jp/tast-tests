// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	fwCommon "chromiumos/tast/common/firmware"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/servo"
	"chromiumos/tast/rpc"
	fwpb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BootMode,
		Desc:         "Verifies that remote tests can boot the DUT into, and confirm that the DUT is in, the different firmware modes (normal, dev, and recovery)",
		Contacts:     []string{"cros-fw-engprod@google.com"},
		Data:         firmware.ConfigDatafiles(),
		ServiceDeps:  []string{"tast.cros.firmware.UtilsService"},
		SoftwareDeps: []string{"crossystem"},
		Attr:         []string{"group:mainline", "informational"},
		Vars:         []string{"servo"},
	})
}

func BootMode(ctx context.Context, s *testing.State) {
	// initialMode is the boot mode which we expect the DUT to start in.
	const initialMode = fwCommon.BootModeNormal
	// modes enumerates the order of BootModes into which this test will reboot the DUT (after initialMode).
	modes := []fwCommon.BootMode{
		fwCommon.BootModeNormal,
	}
	// Connect to the gRPC server on the DUT.
	d := s.DUT()
	cl, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC: ", err)
	}
	// The identity of cl will change as the RPC client reconnects after each reboot.
	// So, defer closing only the most up-to-date cl.
	defer func() {
		if cl != nil {
			cl.Close(ctx)
		}
	}()
	utils := fwpb.NewUtilsServiceClient(cl.Conn)

	// Setup servo.
	pxy, err := servo.NewProxy(ctx, s.RequiredVar("servo"), d.KeyFile(), d.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctx)
	svo := pxy.Servo()

	// Check that DUT starts in initialMode.
	if r, err := utils.CurrentBootMode(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Error during CurrentBootMode at beginning of test: ", err)
	} else if fwCommon.BootModeFromProto[r.BootMode] != initialMode {
		s.Fatalf("DUT was in %s mode at beginning of test; expected %s", fwCommon.BootModeFromProto[r.BootMode], initialMode)
	}

	// Transition through the boot modes enumerated in ms, verifying boot mode at each step along the way.
	fromMode := initialMode
	for _, toMode := range modes {
		testing.ContextLogf(ctx, "Transitioning from %s to %s", fromMode, toMode)
		if err := firmware.RebootToMode(ctx, d, svo, utils, toMode); err != nil {
			s.Fatalf("Error during transition from %s to %s: %+v", fromMode, toMode, err)
		}
		// Reestablish RPC connection after reboot.
		cl.Close(ctx)
		cl, err = rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
		if err != nil {
			s.Fatal("Failed to reconnect to the RPC: ", err)
		}
		utils = fwpb.NewUtilsServiceClient(cl.Conn)
		if r, err := utils.CurrentBootMode(ctx, &empty.Empty{}); err != nil {
			s.Fatalf("Error during CurrentBootMode after transition from %s to %s: %+v", fromMode, toMode, err)
		} else if fwCommon.BootModeFromProto[r.BootMode] != toMode {
			s.Fatalf("DUT was in %s after transition from %s to %s", fwCommon.BootModeFromProto[r.BootMode], fromMode, toMode)
		}
		fromMode = toMode
	}

	// Exercise PowerState, which will later be used during firmware.RebootToMode()
	if err := svo.SetPowerState(ctx, servo.PowerStateOff); err != nil {
		s.Error("Error from setting power state to Off: ", err)
	}
	offCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := d.WaitUnreachable(offCtx); err != nil {
		s.Fatal("Error during d.WaitUnreachable: ", err)
	}
	if err := svo.SetPowerState(ctx, servo.PowerStateOn); err != nil {
		s.Error("Error from setting power state to On: ", err)
	}
	onCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	if err := d.WaitConnect(onCtx); err != nil {
		s.Fatal("Error during d.WaitConnect: ", err)
	}
}
