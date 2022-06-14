// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	fwCommon "chromiumos/tast/common/firmware"
	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/remote/firmware/reporters"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         InvalidUSB,
		Desc:         "Test that corrupting USB test image causes failure during recovery",
		Contacts:     []string{"tij@google.com", "cros-fw-engprod@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable"},
		Vars:         []string{"firmware.skipFlashUSB", "firmware.noVerifyUSB"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		Timeout:      25 * time.Minute,
		Fixture:      fixture.NormalMode,
	})
}

type kernelHeaderMagic string

const (
	chromeOSMagic  kernelHeaderMagic = "CHROMEOS"
	corruptedMagic kernelHeaderMagic = "CORRUPTD"
)

func InvalidUSB(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Failed to require config: ", err)
	}

	if noVerifyUSBStr, ok := s.Var("firmware.skipFlashUSB"); ok {
		noVerifyUSB, err := strconv.ParseBool(noVerifyUSBStr)
		if err != nil {
			s.Fatalf("Invalid value for var firmware.noVerifyUSB: got %q, want true/false", noVerifyUSBStr)
		}
		if !noVerifyUSB {
			skipFlashUSB := false
			if skipFlashUSBStr, ok := s.Var("firmware.skipFlashUSB"); ok {
				skip, err := strconv.ParseBool(skipFlashUSBStr)
				if err != nil {
					s.Fatalf("Invalid value for var firmware.skipFlashUSB: got %q, want true/false", skipFlashUSBStr)
				}
				skipFlashUSB = skip
			}
			cs := s.CloudStorage()
			if skipFlashUSB {
				cs = nil
			}
			if err := h.SetupUSBKey(ctx, cs); err != nil {
				s.Fatal("USBKey not working: ", err)
			}
		}
	}

	usbDev, err := h.Servo.GetStringTimeout(ctx, servo.ImageUSBKeyDev, time.Second*90)
	if err != nil {
		s.Fatal("Servo call image_usbkey_dev failed: ", err)
	} else {
		s.Log("usb is at ", usbDev)
	}

	s.Log("Setting USB mux state to host")
	if err := h.Servo.SetUSBMuxState(ctx, servo.USBMuxHost); err != nil {
		s.Fatal("Failed setting usb mux state to dut: ", err)
	}

	cleanupContext := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 60*time.Minute)
	defer cancel()

	s.Log("Corrupting usb kernel header magic")
	if err := modifyUsbKernel(ctx, h, usbDev, chromeOSMagic, corruptedMagic); err != nil {
		s.Fatal("Failed to corrupt usb kernel header magic: ", err)
	}
	defer func(ctx context.Context) {
		s.Log("Setting USB mux state to host")
		if err := h.Servo.SetUSBMuxState(ctx, servo.USBMuxHost); err != nil {
			s.Fatal("Failed setting usb mux state to dut: ", err)
		}

		s.Log("Restoring usb kernel header magic")
		if err := modifyUsbKernel(ctx, h, usbDev, corruptedMagic, chromeOSMagic); err != nil {
			s.Fatal("Failed to restore usb kernel header magic: ", err)
		}
	}(cleanupContext)

	s.Log("Setting USB mux state to DUT")
	if err := h.Servo.SetUSBMuxState(ctx, servo.USBMuxDUT); err != nil {
		s.Fatal("Failed setting usb mux state to dut: ", err)
	}

	if mainfwType, err := h.Reporter.CrossystemParam(ctx, reporters.CrossystemParamMainfwType); err != nil {
		s.Fatal("Failed to get crossystem mainfw_type: ", err)
	} else if mainfwType != "normal" {
		s.Fatalf("Expected mainfw_type to be 'normal', got %q", mainfwType)
	}
	if devswBoot, err := h.Reporter.CrossystemParam(ctx, reporters.CrossystemParamDevswBoot); err != nil {
		s.Fatal("Failed to get crossystem devsw_boot: ", err)
	} else if devswBoot != "0" {
		s.Fatalf("Expected devsw_boot to be '0', got %q", devswBoot)
	}

	if err := h.Servo.SetPDRole(ctx, servo.PDRoleSnk); err != nil {
		s.Fatal("Failed to set PD role: ", err)
	}

	ms, err := firmware.NewModeSwitcher(ctx, h)
	if err != nil {
		s.Fatal("Failed to create mode switcher: ", err)
	}
	if err := ms.RebootToMode(ctx, fwCommon.BootModeRecovery); err != nil {
		s.Fatal("Failed to boot to go to recovery mode: ", err)
	}

	s.Log("Setting USB mux state to host")
	if err := h.Servo.SetUSBMuxState(ctx, servo.USBMuxHost); err != nil {
		s.Fatal("Failed setting usb mux state to dut: ", err)
	}

	s.Log("Restoring usb kernel header magic")
	if err := modifyUsbKernel(ctx, h, usbDev, corruptedMagic, chromeOSMagic); err != nil {
		s.Fatal("Failed to restore usb kernel header magic: ", err)
	}

	s.Log("Setting USB mux state to DUT")
	if err := h.Servo.SetUSBMuxState(ctx, servo.USBMuxDUT); err != nil {
		s.Fatal("Failed setting usb mux state to dut: ", err)
	}

	if mainfwType, err := h.Reporter.CrossystemParam(ctx, reporters.CrossystemParamMainfwType); err != nil {
		s.Fatal("Failed to get crossystem mainfw_type: ", err)
	} else if mainfwType != "recovery" {
		s.Fatalf("Expected mainfw_type to be 'recovery', got %q", mainfwType)
	}
	recRes, err := h.Reporter.CrossystemParam(ctx, reporters.CrossystemParamRecoveryReason)
	if err != nil {
		s.Fatal("Failed to get crossystem recovery_reason: ", err)
	} else if recRes != reporters.RecoveryReason["ROManual"] {
		s.Fatalf("Expected recovery reason to be ROManual value %q, but got %q", reporters.RecoveryReason["ROManual"], recRes)
	}

	ms, err = firmware.NewModeSwitcher(ctx, h)
	if err != nil {
		s.Fatal("Failed to create mode switcher: ", err)
	}
	if err := ms.ModeAwareReboot(ctx, firmware.ColdReset); err != nil {
		s.Fatal("Failed to perform mode aware reboot: ", err)
	}

	if mainfwType, err := h.Reporter.CrossystemParam(ctx, reporters.CrossystemParamMainfwType); err != nil {
		s.Fatal("Failed to get crossystem mainfw_type: ", err)
	} else if mainfwType != "normal" {
		s.Fatalf("Expected mainfw_type to be 'normal', got %q", mainfwType)
	}
	if devswBoot, err := h.Reporter.CrossystemParam(ctx, reporters.CrossystemParamDevswBoot); err != nil {
		s.Fatal("Failed to get crossystem devsw_boot: ", err)
	} else if devswBoot != "0" {
		s.Fatalf("Expected devsw_boot to be '0', got %q", devswBoot)
	}
}

func modifyUsbKernel(ctx context.Context, h *firmware.Helper, usbDev string, fromMagic, toMagic kernelHeaderMagic) error {
	kernelMap := map[string]int{"a": 2, "b": 4, "2": 2, "4": 4, "3": 2, "5": 4}
	part := kernelMap["a"]
	kernelPart := fmt.Sprintf("%s%d", usbDev, part)
	if strings.Contains(usbDev, "mmcblk") || strings.Contains(usbDev, "nvme") {
		kernelPart = fmt.Sprintf("%sp%d", usbDev, part)
	}
	testing.ContextLog(ctx, "kernel part is at: ", kernelPart)

	out, err := h.ServoProxy.OutputCommand(ctx, true, "dd", fmt.Sprintf("if=%s", kernelPart), "bs=8", "count=1")
	if err != nil {
		return errors.Wrap(err, "dd cmd failed")
	}
	currMagic := kernelHeaderMagic(out)
	testing.ContextLog(ctx, "The current magic is: ", currMagic)
	if currMagic == toMagic {
		testing.ContextLogf(ctx, "The kernel magic is %q which is the same as the requested magic %q", currMagic, toMagic)
		return nil
	} else if currMagic != fromMagic {
		testing.ContextLogf(ctx, "Unexpected kernel image currently on usb, expected magic %q, got %q", fromMagic, currMagic)
	}

	testing.ContextLog(ctx, "Changing kernel magic to: ", string(toMagic))
	fullCmd := fmt.Sprintf("echo -n %s | dd of=%s oflag=sync conv=notrunc", string(toMagic), kernelPart)
	out, err = h.ServoProxy.OutputCommand(ctx, true, "sh", "-c", fullCmd)
	if err != nil {
		return errors.Wrap(err, "failed to write new kernel magic")
	}
	// Verify kernel header magic is changed
	out, err = h.ServoProxy.OutputCommand(ctx, true, "dd", fmt.Sprintf("if=%s", kernelPart), "bs=8", "count=1")
	if err != nil {
		return errors.Wrap(err, "dd cmd failed")
	}
	currMagic = kernelHeaderMagic(out)
	testing.ContextLog(ctx, "The current magic is now: ", currMagic)

	if currMagic != toMagic {
		return errors.Errorf("expected kernel header magic to have changed to %q, is actually %q", toMagic, currMagic)
	}
	return nil
}
