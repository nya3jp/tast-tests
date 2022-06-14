// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

var mapFWWriteProtectStates = map[servo.FWWPStateValue]bool{
	servo.FWWPStateOff: false,
	servo.FWWPStateOn:  true,
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ECSystemLocked,
		Desc:         "This test case verifies that changing the FW write protection state has expected effect",
		Contacts:     []string{"tj@semihalf.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_ec"},
		Fixture:      fixture.DevMode,
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
	})
}

func ECSystemLocked(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servod")
	}

	s.Log("Get initial FW write protect state")
	out, err := h.Servo.GetString(ctx, servo.FWWPState)
	initialState := servo.FWWPStateValue(out)
	if err != nil {
		s.Fatal("Failed to get initial write protect state: ", err)
	}
	defer func() {
		s.Log("Restore the original FW write protect state")
		setFWWriteProtectStateAndReboot(ctx, h, s, mapFWWriteProtectStates[initialState])
	}()
	s.Log("FW initial write protect state: ", initialState)
	setFWWriteProtectStateAndReboot(ctx, h, s, !mapFWWriteProtectStates[initialState])
}

func setFWWriteProtectStateAndReboot(ctx context.Context, h *firmware.Helper, s *testing.State, FWWriteProtectState bool) {
	dut := s.DUT()

	if err := setFWWriteProtectState(ctx, h, FWWriteProtectState); err != nil {
		s.Fatal("Failed to set FW write protect state: ", err)
	}
	s.Log("Rebooting the DUT")
	if err := dut.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot the DUT: ", err)
	}
	waitConnectCtx, cancelWaitConnect := context.WithTimeout(ctx, 2*time.Minute)
	defer cancelWaitConnect()
	if err := h.WaitConnect(waitConnectCtx); err != nil {
		s.Fatal("Failed to reconnect to the DUT: ", err)
	}
	s.Log("Get write protect state after reboot")
	out, err := h.Servo.GetString(ctx, servo.FWWPState)
	stateAfterReboot := servo.FWWPStateValue(out)
	if err != nil {
		s.Fatal("Failed to get write protect state: ", err)
	}
	if mapFWWriteProtectStates[stateAfterReboot] != FWWriteProtectState {
		s.Fatal("Failed to set write protect state to ", stateAfterReboot)
	}
	s.Log("FW write protect state has been successfully set to ", stateAfterReboot)
}

func setFWWriteProtectState(ctx context.Context, h *firmware.Helper, enable bool) error {
	enableStr := "enable"
	fwwpState := servo.FWWPStateOn
	if !enable {
		enableStr = "disable"
		fwwpState = servo.FWWPStateOff
	}

	// Enable software wp before hardware wp if enabling.
	if enable {
		if err := h.Servo.RunECCommand(ctx, "flashwp enable"); err != nil {
			return errors.Wrap(err, "failed to enable flashwp")
		}
	}

	if err := h.Servo.SetFWWPState(ctx, fwwpState); err != nil {
		return errors.Wrapf(err, "failed to %s firmware write protect", enableStr)
	}

	// Disable software wp after hardware wp so its allowed.
	if !enable {
		if err := h.Servo.RunECCommand(ctx, "flashwp disable"); err != nil {
			return errors.Wrap(err, "failed to disable flashwp")
		}
	}

	return nil
}
