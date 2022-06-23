// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"regexp"
	"time"

	fwCommon "chromiumos/tast/common/firmware"
	"chromiumos/tast/common/servo"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/remote/firmware/reporters"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FlashOSViaUSB,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Flash OS through USB 3.0 and boot with Local storage",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"crossystem", "flashrom"},
		Fixture:      fixture.DevMode,
		Timeout:      20 * time.Minute,
		Params: []testing.Param{{
			Name: "type_c",
			Val:  fwCommon.BootModeUSBDev,
		}, {
			Name: "type_a",
			Val:  fwCommon.BootModeRecovery,
		}},
	})
}

func FlashOSViaUSB(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	tc := s.Param().(fwCommon.BootMode)

	var opts []firmware.ModeSwitchOption
	opts = append(opts, firmware.CopyTastFiles)

	s.Log("Enabling USB connection to DUT")
	if err := h.Servo.SetUSBMuxState(ctx, servo.USBMuxDUT); err != nil {
		s.Fatal("Failed to set 'usb3_mux_sel:dut_sees_usbkey': ", err)
	}

	ms, err := firmware.NewModeSwitcher(ctx, h)
	if err != nil {
		s.Fatal("Failed to create new boot mode switcher: ", err)
	}

	s.Logf("Rebooting into %s mode", tc)
	if err := ms.RebootToMode(ctx, tc, opts...); err != nil {
		s.Fatalf("Failed to reboot into %s mode: %v ", tc, err)
	}

	s.Log("Reconnecting to DUT")
	if err := h.WaitConnect(ctx); err != nil {
		s.Fatal("Failed to reconnect to DUT: ", err)
	}

	var expectedDeviceType reporters.BootedDeviceType
	if tc == fwCommon.BootModeRecovery {
		expectedDeviceType = reporters.BootedDeviceRecoveryRemovableSig
	} else {
		expectedDeviceType = reporters.BootedDeviceDeveloperRemovableHash
	}

	s.Log("Checking that DUT has booted from removable device")
	if bootedDeviceType, err := h.Reporter.BootedDevice(ctx); err != nil {
		s.Fatal("Failed to determine boot device type: ", err)
	} else if bootedDeviceType != expectedDeviceType {
		s.Fatalf("DUT did not boot from the removable device, want expectedDeviceType: %v got bootedDeviceType: %v", expectedDeviceType, bootedDeviceType)
	}

	s.Log("Installing Chrome OS")
	installOut, err := h.DUT.Conn().CommandContext(ctx, "chromeos-install", "--y").Output(ssh.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to execute chromeos-install command: ", err)
	}
	sucessRe := regexp.MustCompile(`Installation to '(?:[^\\']|\\\\|\\')*' complete`)
	if !sucessRe.MatchString(string(installOut)) {
		s.Fatal("Failed to verify chrome installation")
	}

	if err := ms.RebootToMode(ctx, fwCommon.BootModeNormal, opts...); err != nil {
		s.Fatal("Failed to reboot into normal mode: ", err)
	}

	s.Log("Reconnecting to DUT")
	if err := h.WaitConnect(ctx); err != nil {
		s.Fatal("Failed to reconnect to DUT: ", err)
	}

	s.Log("Checking that DUT has booted from internal device")
	if bootedDeviceType, err := h.Reporter.BootedDevice(ctx); err != nil {
		s.Fatal("Failed to determine boot device type: ", err)
	} else if bootedDeviceType != reporters.BootedDeviceNormalInternalSig {
		s.Fatalf("DUT did not boot from the internal device, want expectedDeviceType: %v and got bootedDeviceType:%v", reporters.BootedDeviceNormalInternalSig, bootedDeviceType)
	}
}
