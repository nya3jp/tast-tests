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
		s.Fatalf("CurrentBootMode returned BootMode %s; want %s", currentBootModeResponse.BootMode, fwpb.BootMode_BOOT_MODE_NORMAL)
	}
	normalMode, err := firmware.CheckBootMode(ctx, utils, fwCommon.BootModeNormal)
	if err != nil {
		s.Error("Failed calling CheckBootMode RPC wrapper: ", err)
	}
	if !normalMode {
		s.Error("DUT was not in Normal mode at start of test")
	}
	devMode, err := firmware.CheckBootMode(ctx, utils, fwCommon.BootModeDev)
	if err != nil {
		s.Error("Failed calling CheckBootMode RPC wrapper: ", err)
	}
	if devMode {
		s.Error("DUT was thought to be in Dev mode at start of test")
	}
	recMode, err := firmware.CheckBootMode(ctx, utils, fwCommon.BootModeRecovery)
	if err != nil {
		s.Error("Failed calling CheckBootMode RPC wrapper: ", err)
	}
	if recMode {
		s.Error("DUT was thought to be in Rec mode at start of test")
	}

	// Exercise the BlockingSync, which will be used for each mode-switching reboot.
	if _, err := utils.BlockingSync(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Error during BlockingSync: ", err)
	}

	// Exercise loading fw-testing-configs, which is needed for mode-switching reboots.
	platformResponse, err := utils.Platform(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Error during Platform: ", err)
	}
	platform := strings.ToLower(platformResponse.Platform)
	cfg, err := firmware.NewConfig(s.DataPath(firmware.ConfigDir), platform)
	if err != nil {
		s.Fatal("Error during NewConfig: ", err)
	}
	if cfg.Platform != platform {
		s.Fatalf("cfg has Platform %s, want %s: %+v", cfg.Platform, platform, cfg)
	}

	// TODO (gredelston): When we have the ability to reboot the DUT into dev/recovery mode,
	// switch into each mode, and check whether we are in the expected state.
}
