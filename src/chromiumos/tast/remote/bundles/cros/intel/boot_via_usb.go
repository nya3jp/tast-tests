// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package intel

import (
	"context"
	"strings"
	"time"

	fwCommon "chromiumos/tast/common/firmware"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BootViaUSB,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies if DUT can boot from USB Type-C pen drive",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"crossystem", "flashrom"},
		Fixture:      fixture.NormalMode,
		Timeout:      5 * time.Minute,
	})
}

func BootViaUSB(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper

	if err := checkUSBPDMuxInfo(ctx, h.DUT, "USB=1"); err != nil {
		s.Fatal("Failed to find USB Type-C connected to DUT: ", err)
	}

	ms, err := firmware.NewModeSwitcher(ctx, h)
	if err != nil {
		s.Fatal("Failed to create new boot mode switcher: ", err)
	}

	s.Logf("Rebooting into %s mode", fwCommon.BootModeUSBDev)
	if err := ms.RebootToMode(ctx, fwCommon.BootModeUSBDev); err != nil {
		s.Fatalf("Failed to reboot into %s mode: %v ", fwCommon.BootModeUSBDev, err)
	}

	s.Log("Reconnecting to DUT")
	if err := h.WaitConnect(ctx); err != nil {
		s.Fatal("Failed to reconnect to DUT: ", err)
	}

	s.Log("Checking that DUT has booted from removable device")
	bootedFromRemovableDevice, err := h.Reporter.BootedFromRemovableDevice(ctx)
	if err != nil {
		s.Fatal("Failed to determine boot device type: ", err)
	}
	if !bootedFromRemovableDevice {
		s.Fatalf("DUT did not boot from the bootable device: got %v, want true", bootedFromRemovableDevice)
	}
}

// checkUSBPDMuxInfo verifies whether deviceStr is in usbpdmuxinfo or not.
func checkUSBPDMuxInfo(ctx context.Context, dut *dut.DUT, deviceStr string) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		out, err := dut.Conn().CommandContext(ctx, "ectool", "usbpdmuxinfo").Output(testexec.DumpLogOnError)
		if err != nil {
			return errors.Wrap(err, "failed to run usbpdmuxinfo command")
		}
		if !strings.Contains(string(out), deviceStr) {
			return errors.Wrapf(err, "failed to find %s in usbpdmuxinfo", deviceStr)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: 1 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to check deviceStr")
	}
	return nil
}
