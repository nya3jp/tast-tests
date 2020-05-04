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
	// Connect to the gRPC server on the DUT.
	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC: ", err)
	}
	defer cl.Close(ctx)
	utils := fwpb.NewUtilsServiceClient(cl.Conn)

	// DUT should start in normal mode.
	// Exercise all ways of verifying the boot mode.
	currentBootModeResponse, err := utils.CurrentBootMode(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Error during CurrentBootMode: ", err)
	}
	if currentBootModeResponse.BootMode != fwpb.BootMode_BOOT_MODE_NORMAL {
		s.Errorf("CurrentBootMode returned BootMode %s; want %s", currentBootModeResponse.BootMode, fwpb.BootMode_BOOT_MODE_NORMAL)
	}
	normalMode, err := firmware.CheckBootMode(ctx, utils, fwCommon.BootModeNormal)
	if err != nil {
		s.Fatal("Failed calling CheckBootMode RPC wrapper: ", err)
	}
	if !normalMode {
		s.Error("DUT was not in Normal mode at start of test")
	}
	devMode, err := firmware.CheckBootMode(ctx, utils, fwCommon.BootModeDev)
	if err != nil {
		s.Fatal("Failed calling CheckBootMode RPC wrapper: ", err)
	}
	if devMode {
		s.Error("DUT was thought to be in Dev mode at start of test")
	}
	recMode, err := firmware.CheckBootMode(ctx, utils, fwCommon.BootModeRecovery)
	if err != nil {
		s.Fatal("Failed calling CheckBootMode RPC wrapper: ", err)
	}
	if recMode {
		s.Error("DUT was thought to be in Rec mode at start of test")
	}

	// Exercise the BlockingSync, which will be used for each mode-switching reboot.
	if _, err := utils.BlockingSync(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Error during BlockingSync: ", err)
	}

	// Boot from Normal > Normal mode
	d := s.DUT()
	if err := firmware.RebootToMode(ctx, d, utils, fwCommon.BootModeNormal); err != nil {
		s.Fatal("Error during Normal>Normal transition: ", err)
	}

	// Reconnect to the RPC server after rebooting.
	cl, err = rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to reconnect to the RPC: ", err)
	}
	defer cl.Close(ctx)
	utils = fwpb.NewUtilsServiceClient(cl.Conn)

	// Verify that we booted into the new mode correctly
	currentBootModeResponse, err = utils.CurrentBootMode(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Error during CurrentBootMode: ", err)
	}
	if currentBootModeResponse.BootMode != fwpb.BootMode_BOOT_MODE_NORMAL {
		s.Fatalf("After booting Normal>Normal, CurrentBootMode returned BootMode %s; want %s", currentBootModeResponse.BootMode, fwpb.BootMode_BOOT_MODE_NORMAL)
	}
}
