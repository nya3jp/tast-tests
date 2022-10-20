// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"time"

	fwCommon "chromiumos/tast/common/firmware"
	"chromiumos/tast/common/servo"
	fwUtils "chromiumos/tast/remote/bundles/cros/firmware/utils"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/remote/firmware/reporters"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TryFWB,
		Desc:         "Servo based boot firmware B test",
		Contacts:     []string{"pf@semihalf.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable", "firmware_usb"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		Timeout:      10 * time.Minute,
		SoftwareDeps: []string{"crossystem"},
		ServiceDeps:  []string{"tast.cros.firmware.UtilsService"},
		Params: []testing.Param{
			{
				Name:    "normal_mode",
				Fixture: fixture.NormalMode,
				Val:     "normal",
			},
			{
				Name:    "dev_mode",
				Fixture: fixture.DevModeGBB,
				Val:     "developer",
			},
		},
	})
}

func TryFWB(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to init servo: ", err)
	}

	ms, err := firmware.NewModeSwitcher(ctx, h)
	if err != nil {
		s.Fatal("Creating mode switcher: ", err)
	}

	s.Log("Set the USB Mux direction to Host")
	if err := h.Servo.SetUSBMuxState(ctx, servo.USBMuxHost); err != nil {
		s.Fatal(err, "failed to set the USB Mux direction to the Host")
	}

	vboot2, err := h.Reporter.Vboot2(ctx)
	if err != nil {
		s.Fatal("Failed to determine fw_vboot2: ", err)
	}

	if !vboot2 {
		if triedFWB, err := h.Reporter.CrossystemParam(ctx, reporters.CrossystemParamTriedFWB); err != nil {
			s.Fatal("Failed to read the tried_fwb param: ", err)
		} else if triedFWB != "0" {
			s.Log("Firmware is booted with tried_fwb. Reboot to clear")
		}
	}

	s.Log("Start test with FW A")
	if err := fwUtils.ChangeFWVariant(ctx, h, ms, fwCommon.RWSectionA); err != nil {
		s.Fatal("Failed to change FW variant: ", err)
	}

	s.Log("Switch firmware to B variant, reboot")
	if err := fwUtils.ChangeFWVariant(ctx, h, ms, fwCommon.RWSectionB); err != nil {
		s.Fatal("Failed to change FW variant: ", err)
	}

	if err := ms.ModeAwareReboot(ctx, firmware.WarmReset); err != nil {
		s.Fatal("Failed to perform mode aware reboot: ", err)
	}

	finalFWVer := "A"
	if vboot2 {
		finalFWVer = "B"
	}

	if isFWVerCorrect, err := h.Reporter.CheckFWVersion(ctx, finalFWVer); err != nil {
		s.Fatal(err, "failed to check a firmware version")
	} else if !isFWVerCorrect {
		s.Fatalf("Failed to boot into the %s firmware", finalFWVer)
	}
}
