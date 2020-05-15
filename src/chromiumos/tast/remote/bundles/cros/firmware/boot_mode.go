// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"strings"

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
		fwCommon.BootModeRecovery,
		fwCommon.BootModeRecovery,
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

	// Setup Config.
	platformResponse, err := utils.Platform(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Error during Platform: ", err)
	}
	board := strings.ToLower(platformResponse.Board)
	model := strings.ToLower(platformResponse.Model)
	cfg, err := firmware.NewConfig(s.DataPath(firmware.ConfigDir), board, model)
	if err != nil {
		s.Fatal("Error during NewConfig: ", err)
	}

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
		if err := firmware.RebootToMode(ctx, d, svo, cfg, utils, toMode); err != nil {
			s.Errorf("Error during transition from %s to %s: %+v", fromMode, toMode, err)
			break
		}
		// Reestablish RPC connection after reboot.
		cl.Close(ctx)
		testing.ContextLog(ctx, "Reconnecting to RPC")
		cl, err = rpc.Dial(ctx, d, s.RPCHint(), "cros")
		if err != nil {
			s.Fatal("Failed to reconnect to the RPC: ", err)
		}
		utils = fwpb.NewUtilsServiceClient(cl.Conn)
		if r, err := utils.CurrentBootMode(ctx, &empty.Empty{}); err != nil {
			s.Errorf("Error during CurrentBootMode after transition from %s to %s: %+v", fromMode, toMode, err)
			break
		} else if fwCommon.BootModeFromProto[r.BootMode] != toMode {
			s.Errorf("DUT was in %s after transition from %s to %s", fwCommon.BootModeFromProto[r.BootMode], fromMode, toMode)
			break
		}
		fromMode = toMode
	}

	r, err := utils.CurrentBootMode(ctx, &empty.Empty{})
	if err != nil {
		s.Error("Error getting boot mode at end of test: ", err)
	} else if fwCommon.BootModeFromProto[r.BootMode] != initialMode {
		testing.ContextLog(ctx, "Resetting mode to initialMode")
		if err := firmware.RebootToMode(ctx, d, svo, cfg, utils, initialMode); err != nil {
			s.Error("Error rebooting to initialMode at end of test: ", err)
		}
	}
}
