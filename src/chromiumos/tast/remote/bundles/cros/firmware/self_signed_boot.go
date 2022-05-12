// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	common "chromiumos/tast/common/firmware"
	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/remote/firmware/reporters"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SelfSignedBoot,
		Desc:         "TODO(tij@) FILL THIS OUT",
		Contacts:     []string{"tij@google.com", "cros-fw-engprod@google.com"},
		Vars:         []string{"firmware.skipFlashUSB", "firmware.noVerifyUSB"},
		Attr:         []string{"group:firmware", "firmware_unstable"},
		SoftwareDeps: []string{"crossystem", "flashrom"},
		Fixture:      fixture.DevMode,
		Timeout:      25 * time.Minute,
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.Keyboard()),
	})
}

func SelfSignedBoot(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	if noVerifyUSBStr, ok := s.Var("firmware.skipFlashUSB"); ok {
		noVerifyUSB, err := strconv.ParseBool(noVerifyUSBStr)
		if err != nil {
			s.Fatalf("Invalid value for var firmware.noVerifyUSB: got %q, want true/false", noVerifyUSBStr)
		}
		if !noVerifyUSB {
			s.Log("wth")
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

	// Get USB Device
	out, err := h.DUT.Conn().CommandContext(ctx, "rootdev", "-s").Output(ssh.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to check rootdev value: ", err)
	}
	rootDev := strings.TrimSpace(string(out))
	usbDev := regexp.MustCompile(`p?\d+$`).ReplaceAllString(rootDev, "")
	testing.ContextLog(ctx, "usbdev path is: ", usbDev)

	cleanupContext := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Minute)
	defer cancel()
	defer func(ctx context.Context) {
		if err := setSelfSigned(ctx, h, false); err != nil {
			s.Fatal("Failed to set self signed state to disable")
		}

		s.Log("Resign usb image with ssd keys")
		if _, err := h.DUT.Conn().CommandContext(ctx, "/usr/share/vboot/bin/make_dev_ssd.sh", "--partitions", "2", "-i", usbDev, "--recovery_key").Output(ssh.DumpLogOnError); err != nil {
			s.Fatal("Failed to set dev_boot_signed_only to 1: ", err)
		}

	}(cleanupContext)

	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	if err := checkDevBootUSB(ctx, h, usbDev, "sig"); err != nil {
		s.Fatal("Failed to get crossystem params: ", err)
	}

	if err := setSelfSigned(ctx, h, true); err != nil {
		s.Fatal("Failed to set self signed state to enable: ", err)
	}

	ms, err := firmware.NewModeSwitcher(ctx, h)
	if err != nil {
		s.Fatal("Failed to create mode switcher: ", err)
	}

	s.Log("Performing mode aware reboot")
	if err := ms.ModeAwareReboot(ctx, firmware.ColdReset); err != nil {
		s.Fatal("Failed to perform mode aware reboot: ", err)
	}

	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	if err := checkDevBootUSB(ctx, h, usbDev, "sig"); err != nil {
		s.Fatal("Failed to check crossystem params: ", err)
	}

	s.Log("Reboot to recovery")
	if err := ms.RebootToMode(ctx, common.BootModeRecovery); err != nil {
		s.Fatal("Failed to boot to rec mode: ", err)
	}

	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	mainfwType, err := h.Reporter.CrossystemParam(ctx, reporters.CrossystemParamMainfwType)
	if err != nil {
		s.Fatal("Failed to get crossystem mainfw_type: ", err)
	} else if mainfwType != "recovery" {
		s.Fatalf("Expected mainfw_type to be 'recovery', got %q", mainfwType)
	}

	recRes, err := h.Reporter.CrossystemParam(ctx, reporters.CrossystemParamRecoveryReason)
	if err != nil {
		s.Fatal("Failed to get crossystem recovery_reason: ", err)
	} else if reporters.RecoveryReason(recRes) != reporters.ROManual {
		s.Fatalf("Expected recovery reason to be ROManual value %s, but got %s", reporters.ROManual, recRes)
	}

	s.Log("Performing mode aware reboot")
	if err := ms.ModeAwareReboot(ctx, firmware.ColdReset); err != nil {
		s.Fatal("Failed to perform mode aware reboot: ", err)
	}

	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	if err := checkDevBootUSB(ctx, h, usbDev, "hash"); err != nil {
		s.Fatal("Failed to check crossystem params: ", err)
	}

	s.Log("Resign usb image with ssd keys")
	if _, err := h.DUT.Conn().CommandContext(ctx, "/usr/share/vboot/bin/make_dev_ssd.sh", "--partitions", "2", "-i", usbDev).Output(ssh.DumpLogOnError); err != nil {
		s.Fatal("Failed to resign usb image with ssd keys: ", err)
	}

	if err := h.Servo.SetPowerState(ctx, servo.PowerStateReset); err != nil {
		s.Fatal("Failed to perform simple reboot: ", err)
	}

	if err := ms.FwScreenToUSBDevMode(ctx); err != nil {
		s.Fatal("Failed to bypass dev boot usb: ", err)
	}

	if err := h.DUT.WaitConnect(ctx); err != nil {
		s.Fatal("Failed to wait for DUT to connect: ", err)
	}

	// After signing USB with SSD dev keys, kernkey_vfy value expected as 'sig' when booted in USB image.
	if err := checkDevBootUSB(ctx, h, usbDev, "sig"); err != nil {
		s.Fatal("Failed to check crossystem params: ", err)
	}

	s.Log("Performing mode aware reboot")
	if err := ms.ModeAwareReboot(ctx, firmware.ColdReset); err != nil {
		s.Fatal("Failed to perform mode aware reboot: ", err)
	}

	if err := checkDevBootUSB(ctx, h, usbDev, "hash"); err != nil {
		s.Fatal("Failed to check crossystem params: ", err)
	}
}

func setSelfSigned(ctx context.Context, h *firmware.Helper, enable bool) error {
	setVal := "1"
	if !enable {
		setVal = "0"
	}

	testing.ContextLogf(ctx, "Set dev_boot_signed_only %q", setVal)
	if _, err := h.DUT.Conn().CommandContext(ctx, "crossystem", fmt.Sprintf("dev_boot_signed_only=%s", setVal)).Output(ssh.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "failed to set dev_boot_signed_only=%s", setVal)
	}

	testing.ContextLogf(ctx, "Set dev_boot_usb %q", setVal)
	if _, err := h.DUT.Conn().CommandContext(ctx, "crossystem", fmt.Sprintf("dev_boot_usb=%s", setVal)).Output(ssh.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "failed to set dev_boot_usb=%s", setVal)
	}
	return nil
}

func checkDevBootUSB(ctx context.Context, h *firmware.Helper, usbDev, expKernkey string) error {
	mainfwType, err := h.Reporter.CrossystemParam(ctx, reporters.CrossystemParamMainfwType)
	if err != nil {
		return errors.Wrap(err, "failed to get crossystem mainfw_type")
	}
	kernkeyVfy, err := h.Reporter.CrossystemParam(ctx, reporters.CrossystemParamKernkeyVfy)
	if err != nil {
		return errors.Wrap(err, "failed to get crossystem kernkey_vfy")
	}

	testing.ContextLogf(ctx, "mainfw_type is: %v, kernkey_vfy is: %v", mainfwType, kernkeyVfy)
	if mainfwType != "developer" {
		return errors.Errorf("expected mainfw_type to be 'developer', got %q", mainfwType)
	} else if kernkeyVfy != expKernkey {
		return errors.Errorf("expected kernkey_vfy to be %q, got %q", expKernkey, kernkeyVfy)
	}

	// Extract just the device, ignore the path.
	rootDev := strings.Split(usbDev, "/")[2]
	out, err := h.DUT.Conn().CommandContext(ctx, "cat", fmt.Sprintf("/sys/block/%s/removable", rootDev)).Output(ssh.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "failed to check rootdev value")
	}

	if string(out) == "1" {
		return errors.New("expected dev_boot_usb to not be removable, but found it was")
	}

	return nil
}
