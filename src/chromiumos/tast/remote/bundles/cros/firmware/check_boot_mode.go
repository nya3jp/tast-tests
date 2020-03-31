// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"reflect"

	"github.com/golang/protobuf/ptypes/empty"

	fwCommon "chromiumos/tast/common/firmware"
	"chromiumos/tast/remote/firmware"
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
	h := firmware.NewHelper(s.DUT())
	defer h.Close(ctx)
	if err := h.AddRPC(ctx, s.RPCHint()); err != nil {
		s.Error("registering rpc to helper: ")
	}
	if err := h.AddConfig(ctx, s.DataPath("fw-testing-configs")); err != nil {
		s.Error("registering config to helper: ", err)
	}

	// DUT should start in normal mode.
	// Exercise all ways of verifying the boot mode.
	currentBootModeResponse, err := h.Utils.CurrentBootMode(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Error during CurrentBootMode: ", err)
	}
	if currentBootModeResponse.BootMode != fwpb.BootMode_BOOT_MODE_NORMAL {
		s.Fatalf("CurrentBootMode returned BootMode %s; want %s", currentBootModeResponse.BootMode, fwpb.BootMode_BOOT_MODE_NORMAL)
	}
	normalMode, err := firmware.CheckBootMode(ctx, h.Utils, fwCommon.BootModeNormal)
	if err != nil {
		s.Error("Failed calling CheckBootMode RPC wrapper: ", err)
	}
	if !normalMode {
		s.Error("DUT was not in Normal mode at start of test")
	}
	devMode, err := firmware.CheckBootMode(ctx, h.Utils, fwCommon.BootModeDev)
	if err != nil {
		s.Error("Failed calling CheckBootMode RPC wrapper: ", err)
	}
	if devMode {
		s.Error("DUT was thought to be in Dev mode at start of test")
	}
	recMode, err := firmware.CheckBootMode(ctx, h.Utils, fwCommon.BootModeRecovery)
	if err != nil {
		s.Error("Failed calling CheckBootMode RPC wrapper: ", err)
	}
	if recMode {
		s.Error("DUT was thought to be in Rec mode at start of test")
	}

	// Exercise the BlockingSync, which will be used for each mode-switching reboot.
	if _, err := h.Utils.BlockingSync(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Error during BlockingSync: ", err)
	}

	// Exercise the RPC to get the platform name, which will be used to get config info needed for mode-switching reboots.
	platformResponse, err := h.Utils.Platform(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Error during Platform: ", err)
	}
	s.Logf("Platform name: %s", platformResponse.Platform)

	// Exercise the creation of the config struct, which will be needed for mode-switching reboots.
	expectedConfig := &firmware.Config{
		ModeSwitcherType:     firmware.KeyboardDevSwitcher,
		PowerButtonDevSwitch: false,
		RecButtonDevSwitch:   false,
		FirmwareScreen:       10,
		DelayRebootToPing:    30,
		ConfirmScreen:        3,
		USBPlug:              10,
	}
	if !reflect.DeepEqual(h.Config, expectedConfig) {
		s.Fatalf("NewConfig produced %+v, unequal to expected %+v", h.Config, expectedConfig)
	}

	// TODO (gredelston): When we have the ability to reboot the DUT into dev/recovery mode,
	// switch into each mode, and check whether we are in the expected state.
}
