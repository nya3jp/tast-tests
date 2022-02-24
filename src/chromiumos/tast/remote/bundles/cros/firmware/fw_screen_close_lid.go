// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"fmt"
	"strings"
	"time"

	fwCommon "chromiumos/tast/common/firmware"
	"chromiumos/tast/common/servo"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FwScreenCloseLid,
		Desc:         "Verify lid close shutdowns machine in states like developer, recovery, usb recovery screens",
		Contacts:     []string{"tij@google.com", "cros-fw-engprod@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.Lid(), hwdep.InternalDisplay()),
		Timeout:      75 * time.Minute,
		Fixture:      fixture.DevMode,
	})
}

type kernelHeaderMagic string

const (
	chromeOSMagic  kernelHeaderMagic = "CHROMEOS"
	corruptedMagic kernelHeaderMagic = "CORRUPTD"
)

func FwScreenCloseLid(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Failed to require config: ", err)
	}

	s.Log("Setting up USB key (will take several minutes)")
	if err := h.SetupUSBKey(ctx, s.CloudStorage()); err != nil {
		s.Fatal("USBKey not working: ", err)
	}

	usbDev, err := h.Servo.GetStringTimeout(ctx, servo.ImageUSBKeyDev, time.Second*90)
	if err != nil {
		s.Fatal("Servo call image_usbkey_dev failed: ", err)
	} else {
		s.Log("usb is at ", usbDev)
	}

	s.Log("Corrupting usb kernel header magic")
	if err := modifyUsbKernel(ctx, h, usbDev, chromeOSMagic, corruptedMagic); err != nil {
		s.Fatal("Failed to corrupt usb kernel header magic: ", err)
	}
	defer func() {
		s.Log("Restoring usb kernel header magic")
		if err := modifyUsbKernel(ctx, h, usbDev, corruptedMagic, chromeOSMagic); err != nil {
			s.Fatal("Failed to corrupt usb kernel header magic: ", err)
		}
	}()

	shortShutdownConfDur := 100 * time.Millisecond

	s.Log("Setting power state to warm reset")
	if err := h.Servo.SetPowerState(ctx, servo.PowerStateWarmReset); err != nil {
		s.Fatalf("Failed setting power state to %s: %v", servo.PowerStateWarmReset, err)
	}
	if err := closeOpenLidReboot(ctx, h, h.Config.FirmwareScreen, h.Config.Shutdown, fwCommon.BootModeDev); err != nil {
		s.Fatal("foo failed: ", err)
	}

	s.Log("Setting power state to warm reset")
	if err := h.Servo.SetPowerState(ctx, servo.PowerStateWarmReset); err != nil {
		s.Fatalf("Failed setting power state to %s: %v", servo.PowerStateWarmReset, err)
	}
	s.Log("Go from fw screen to dev mode")
	if ms, err := firmware.NewModeSwitcher(ctx, h); err != nil {
		s.Fatal("Failed to create mode switcher: ", err)
	} else {
		if err := ms.FwScreenToDevMode(ctx, false, false); err != nil {
			s.Fatal("Failed to boot to go to dev mode: ", err)
		}
	}
	if err := closeOpenLidReboot(ctx, h, h.Config.FirmwareScreen, shortShutdownConfDur, fwCommon.BootModeDev); err != nil {
		s.Fatal("foo failed: ", err)
	}

	s.Log("Request recovery next reboot")
	if err := h.DUT.Conn().CommandContext(ctx, "crossystem", "recovery_request=193").Run(ssh.DumpLogOnError); err != nil {
		s.Fatal("Failed to request recovery next reboot: ", err)
	}
	s.Log("Setting power state to warm reset")
	if err := h.Servo.SetPowerState(ctx, servo.PowerStateWarmReset); err != nil {
		s.Fatalf("Failed setting power state to %s: %v", servo.PowerStateWarmReset, err)
	}
	if err := closeOpenLidReboot(ctx, h, 2*h.Config.FirmwareScreen, shortShutdownConfDur, fwCommon.BootModeDev); err != nil {
		s.Fatal("foo failed: ", err)
	}

	s.Log("Request recovery next reboot")
	if err := h.DUT.Conn().CommandContext(ctx, "crossystem", "recovery_request=193").Run(ssh.DumpLogOnError); err != nil {
		s.Fatal("Failed to request recovery next reboot: ", err)
	}
	s.Log("Setting USB mux state to off")
	if err := h.Servo.SetUSBMuxState(ctx, servo.USBMuxDUT); err != nil {
		s.Fatal("Failed setting usb mux state to dut: ", err)
	}
	s.Logf("Sleeping for %s waiting for usb plug", h.Config.USBPlug)
	if err := testing.Sleep(ctx, h.Config.USBPlug); err != nil {
		s.Fatalf("Failed to sleep for %s waiting for usb plug", h.Config.USBPlug)
	}
	s.Log("Setting power state to warm reset")
	if err := h.Servo.SetPowerState(ctx, servo.PowerStateWarmReset); err != nil {
		s.Fatalf("Failed setting power state to %s: %v", servo.PowerStateWarmReset, err)
	}
	if err := closeOpenLidReboot(ctx, h, 2*h.Config.FirmwareScreen, shortShutdownConfDur, fwCommon.BootModeDev); err != nil {
		s.Fatal("foo failed: ", err)
	}

	s.Log("Reboot to normal mode")
	if ms, err := firmware.NewModeSwitcher(ctx, h); err != nil {
		s.Fatal("Failed to create mode switcher: ", err)
	} else {
		if err := ms.RebootToMode(ctx, fwCommon.BootModeNormal); err != nil {
			s.Fatal("Failed to boot to normal mode: ", err)
		}
	}

	s.Log("Request recovery next reboot")
	if err := h.DUT.Conn().CommandContext(ctx, "crossystem", "recovery_request=193").Run(ssh.DumpLogOnError); err != nil {
		s.Fatal("Failed to request recovery next reboot: ", err)
	}
	s.Log("Setting power state to warm reset")
	if err := h.Servo.SetPowerState(ctx, servo.PowerStateWarmReset); err != nil {
		s.Fatalf("Failed setting power state to %s: %v", servo.PowerStateWarmReset, err)
	}
	if err := closeOpenLidReboot(ctx, h, 2*h.Config.FirmwareScreen, shortShutdownConfDur, fwCommon.BootModeNormal); err != nil {
		s.Fatal("foo failed: ", err)
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

	out, _ := h.DUT.Conn().CommandContext(ctx, "dd", fmt.Sprintf("if=%s", kernelPart), "bs=8", "count=1").Output(ssh.DumpLogOnError)
	currMagic := kernelHeaderMagic(out)
	testing.ContextLog(ctx, "The current magic is: ", currMagic)
	if currMagic == toMagic {
		testing.ContextLogf(ctx, "The kernel magic is %q which is the same as the requested magic %q", currMagic, toMagic)
		return nil
	} else if currMagic != fromMagic {
		return errors.Errorf("unexpected kernel image currently on usb, expected magic %q, got %q", fromMagic, currMagic)
	}

	echoCmd := h.DUT.Conn().CommandContext(ctx, "echo", "-n", string(toMagic))
	// dd of=%s oflag=sync conv=notrunc 2>/dev/null"
	ddCmd := h.DUT.Conn().CommandContext(ctx, "dd", fmt.Sprintf("of=%s", kernelPart), "oflag=sync", "conv=notrunc")
	ddCmd.Stdin, _ = echoCmd.StdoutPipe()
	if err := ddCmd.Start(); err != nil {
		return errors.Wrap(err, "failed to start dd cmd")
	}
	if err := echoCmd.Run(); err != nil {
		return errors.Wrap(err, "failed to run echo cmd")
	}
	ddCmd.Wait()

	// Verify kernel header magic is changed
	out, _ = h.DUT.Conn().CommandContext(ctx, "dd", fmt.Sprintf("if=%s", kernelPart), "bs=8", "count=1").Output(ssh.DumpLogOnError)
	currMagic = kernelHeaderMagic(out)
	testing.ContextLog(ctx, "The current magic is now: ", currMagic)

	if currMagic != toMagic {
		return errors.Errorf("expected kernel header magic to have changed to %q, is actually %q", toMagic, currMagic)
	}
	return nil
}

func closeOpenLidReboot(ctx context.Context, h *firmware.Helper, fwScreenWait, shutdownTimeout time.Duration, bootMode fwCommon.BootMode) error {
	testing.ContextLogf(ctx, "Sleeping for %s to get to fw screen", fwScreenWait)
	if err := testing.Sleep(ctx, fwScreenWait); err != nil {
		return errors.Wrapf(err, "failed to sleep for %s waiting for fw screen", fwScreenWait)
	}

	if err := h.Servo.CloseLid(ctx); err != nil {
		return errors.Wrap(err, "failed to close lid")
	}

	testing.ContextLog(ctx, "Checking for S5 or G3 powerstate for: ", shutdownTimeout)
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, shutdownTimeout, "S5", "G3"); err != nil {
		return errors.Wrap(err, "failed to get S5 or G3 powerstate")
	}

	if err := h.Servo.OpenLid(ctx); err != nil {
		return errors.Wrap(err, "failed to open lid")
	}

	if !h.Config.LidWakeFromPowerOff {
		if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.Dur(h.Config.HoldPwrButtonPowerOn)); err != nil {
			return errors.Wrap(err, "failed to press power key to power on DUT")
		}
	}

	ms, err := firmware.NewModeSwitcher(ctx, h)
	if err != nil {
		return errors.Wrap(err, "failed to create mode switcher")
	}

	bootModeFunc := ms.FwScreenToDevMode
	if bootMode == fwCommon.BootModeNormal {
		bootModeFunc = ms.FwScreenToNormalMode
	}

	testing.ContextLog(ctx, "Go from fw screen to dev mode")
	if err := bootModeFunc(ctx, false, false); err != nil {
		return errors.Wrap(err, "failed to boot to go to dev mode")
	}

	testing.ContextLog(ctx, "Verifying current boot mode is ", bootMode)
	if currMode, err := h.Reporter.CurrentBootMode(ctx); err != nil {
		return errors.Wrap(err, "checking boot mode after rebooting")
	} else if currMode != bootMode {
		return errors.Errorf("incorrect boot mode after resetting DUT: got %s; want %s", currMode, bootMode)
	} else {
		testing.ContextLogf(ctx, "The current boot mode is %s", currMode)
	}

	return h.WaitConnect(ctx)
}
