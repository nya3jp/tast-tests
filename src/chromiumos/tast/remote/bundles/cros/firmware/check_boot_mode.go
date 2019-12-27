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
		Func:        CheckBootMode,
		Desc:        "Verifies that remote tests can check whether the DUT is in normal, dev, and recovery mode",
		Contacts:    []string{"chromeos-engprod@google.com"},
		ServiceDeps: []string{"tast.cros.firmware.UtilsService"},
		Attr:        []string{"group:mainline", "informational"},
	})
}

func CheckBootMode(ctx context.Context, s *testing.State) {
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

	// isNormal, isDev, and isRecovery are lightweight wrappers around checkBootMode.
	isNormal := func() bool {
		return checkBootMode(fwpb.BootMode_BOOT_MODE_NORMAL)
	}
	isDev := func() bool {
		return checkBootMode(fwpb.BootMode_BOOT_MODE_DEV)
	}
	isRecovery := func() bool {
		return checkBootMode(fwpb.BootMode_BOOT_MODE_RECOVERY)
	}

	// DUT should start in normal mode.
	// Exercise both positive and negative checks.
	if !isNormal() {
		s.Error("DUT was not in Normal mode at start of test")
	}
	if isDev() {
		s.Error("DUT was thought to be in Dev mode at start of test")
	}
	if isRecovery() {
		s.Error("DUT was thought to be in Recovery mode at start of test")
	}

	// TODO (gredelston): When we have the ability to reboot the DUT into dev/recovery mode,
	// switch into each mode, and check whether we are in the expected state.
}
