// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"

	"chromiumos/tast/rpc"
	fwpb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        SwitchBootMode,
		Desc:        "Verifies that system can transition between normal, dev, and recovery mode",
		Contacts:    []string{"tast-owners@google.com"},
		ServiceDeps: []string{"tast.cros.firmware.UtilsService"},
		Attr:        []string{"group:mainline", "informational"},
	})
}

func SwitchBootMode(ctx context.Context, s *testing.State) {
	// Connect to the gRPC server on the DUT.
	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC: ", err)
	}
	defer cl.Close(ctx)
	utils := fwpb.NewUtilsServiceClient(cl.Conn)

	// checkBootMode wraps an RPC call to check whether the DUT is in the specified mode.
	checkBootMode := func(bootMode fwpb.BootMode) bool {
		req := &fwpb.CheckBootModeRequest{BootMode: bootMode}
		res, err := utils.CheckBootMode(ctx, req)
		if err != nil {
			s.Fatalf("Error when calling fwpb.CheckBootMode(%s): %s", bootMode, err)
		}
		return res.GetVerified()
	}

	// DUT should start in normal mode.
	isNormal := checkBootMode(fwpb.BootMode_BOOT_MODE_NORMAL)
	if !isNormal {
		s.Error("DUT was not in Normal mode at start of test")
	}

	// Exercising checking for dev mode and recovery mode,
	// as well as checking for a mode that should return false.
	isDev := checkBootMode(fwpb.BootMode_BOOT_MODE_DEV)
	if isDev {
		s.Error("DUT was thought to be in Dev mode at start of test")
	}
	isRec := checkBootMode(fwpb.BootMode_BOOT_MODE_RECOVERY)
	if isRec {
		s.Error("DUT was thought to be in Recovery mode at start of test")
	}

	// TODO (gredelston): Test state transitions:
	// Normal > Dev
	// Dev > Rec
	// Rec > Normal
}
