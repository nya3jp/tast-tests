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

// modes enumerates the order in which the DUT will transition through BootModes during this test.
var modes = []fwCommon.BootMode{
	fwCommon.BootModeNormal,
	fwCommon.BootModeNormal,
}

func CheckBootMode(ctx context.Context, s *testing.State) {
	// Connect to the gRPC server on the DUT.
	d := s.DUT()
	cl, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC: ", err)
	}
	defer cl.Close(ctx)
	utils := fwpb.NewUtilsServiceClient(cl.Conn)

	// DUT should start in normal mode.
	if r, err := utils.CurrentBootMode(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Error during CheckBootMode at beginning of test: ", err)
	} else if fwCommon.BootModeFromProto[r.BootMode] != fwCommon.BootModeNormal {
		s.Fatalf("DUT was in %s mode at beginning of test; expected normal", fwCommon.BootModeFromProto[r.BootMode])
	}

	// Transition through the boot modes enumerated in ms, verifying boot mode at each step along the way.
	var fromMode, toMode fwCommon.BootMode
	for i := 0; i < len(modes)-1; i++ {
		fromMode = modes[i]
		toMode = modes[i]
		testing.ContextLogf(ctx, "Transitioning from %s to %s", fromMode, toMode)
		if err := firmware.RebootToMode(ctx, d, utils, toMode); err != nil {
			s.Fatalf("Error during transition from %s to %s: %+v", fromMode, toMode, err)
		}
		// Rebooting kills the RPC connection.
		cl, err = rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
		if err != nil {
			s.Fatal("Failed to reconnect to the RPC: ", err)
		}
		defer cl.Close(ctx)
		utils = fwpb.NewUtilsServiceClient(cl.Conn)
		if r, err := utils.CurrentBootMode(ctx, &empty.Empty{}); err != nil {
			s.Fatalf("Error during CurrentBootMode after transition from %s to %s: %+v", fromMode, toMode, err)
		} else if fwCommon.BootModeFromProto[r.BootMode] != toMode {
			s.Fatalf("DUT was in %s after transition from %s to %s", fwCommon.BootModeFromProto[r.BootMode], fromMode, toMode)
		}
	}
}
