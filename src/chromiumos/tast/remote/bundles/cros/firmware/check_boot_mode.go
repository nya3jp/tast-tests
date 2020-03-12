// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"reflect"

	"github.com/golang/protobuf/ptypes/empty"

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

	// DUT should start in normal mode.
	// Exercise both positive and negative checks.
	if !checkBootMode(fwpb.BootMode_BOOT_MODE_NORMAL) {
		s.Error("DUT was not in Normal mode at start of test")
	}
	if checkBootMode(fwpb.BootMode_BOOT_MODE_DEV) {
		s.Error("DUT was thought to be in Dev mode at start of test")
	}
	if checkBootMode(fwpb.BootMode_BOOT_MODE_RECOVERY) {
		s.Error("DUT was thought to be in Recovery mode at start of test")
	}

	// Exercise the BlockingSync, which will be used for each mode-switching reboot.
	if _, err := utils.BlockingSync(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Error during BlockingSync: ", err)
	}

	// Exercise the RPC to get the platform name, which will be used to get config info needed for mode-switching reboots.
	r, err := utils.Platform(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Error during Platform: ", err)
	}
	s.Logf("Platform name: %s", r.Platform)

	// Exercise the creation of the config struct, which will be needed for mode-switching reboots.
	c, err := firmware.NewConfig()
	if err != nil {
		s.Fatal("Error during NewConfig: ", err)
	}
	expectedConfig := &firmware.Config{firmware.KeyboardDevSwitcher, false, false, 10, 30, 3, 10}
	if !reflect.DeepEqual(c, expectedConfig) {
		s.Fatalf("NewConfig produced %+v, unequal to expected %+v", c, expectedConfig)
	}

	// TODO (gredelston): When we have the ability to reboot the DUT into dev/recovery mode,
	// switch into each mode, and check whether we are in the expected state.
}
