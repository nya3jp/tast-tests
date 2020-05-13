// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"

	fwCommon "chromiumos/tast/common/firmware"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/rpc"
	fwpb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        CheckBootMode,
		Desc:        "Verifies that remote tests can check whether the DUT is in normal, dev, and recovery mode",
		Contacts:    []string{"chromeos-engprod@google.com"},
		Data:        firmware.ConfigDatafiles(),
		ServiceDeps: []string{"tast.cros.firmware.UtilsService"},
		Attr:        []string{"group:mainline", "informational"},
	})
}

func CheckBootMode(ctx context.Context, s *testing.State) {
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

	// Check that DUT starts in initialMode.
	if r, err := utils.CurrentBootMode(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Error during CheckBootMode at beginning of test: ", err)
	} else if fwCommon.BootModeFromProto[r.BootMode] != initialMode {
		s.Fatalf("DUT was in %s mode at beginning of test; expected %s", fwCommon.BootModeFromProto[r.BootMode], initialMode)
	}

	// Transition through the boot modes enumerated in ms, verifying boot mode at each step along the way.
	fromMode := initialMode
	for _, toMode := range modes {
		testing.ContextLogf(ctx, "Transitioning from %s to %s", fromMode, toMode)
		if err := firmware.RebootToMode(ctx, d, utils, toMode); err != nil {
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
}
