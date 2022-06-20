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

	state, err := h.Servo.GetString(ctx, servo.FWWPState)
	if err != nil {
		s.Fatal("Failed to get initial write protect state: ", err)
	}
	initialState, err := verifyFWWriteProtectState(state)
	if err != nil {
		s.Fatal("Failed to recognize initial FW WP state: ", err)
	}
	defer func() {
		s.Log("Restore the original FW write protect state")
		setFWWriteProtectStateAndReboot(ctx, h, s, initialState)
	}()
	s.Log("FW initial write protect state: ", state)
	setFWWriteProtectStateAndReboot(ctx, h, s, !initialState)
}

func setFWWriteProtectStateAndReboot(ctx context.Context, h *firmware.Helper, s *testing.State, newFWWriteProtectState bool) {
	dut := s.DUT()

	if err := setFWWriteProtectState(ctx, h, newFWWriteProtectState); err != nil {
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
	state, err := h.Servo.GetString(ctx, servo.FWWPState)
	if err != nil {
		s.Fatal("Failed to get write protect state: ", err)
	}
	stateAfterReboot, err := verifyFWWriteProtectState(state)
	if err != nil {
		s.Fatal("Failed to recognize FW WP state after reboot: ", err)
	}
	if stateAfterReboot != newFWWriteProtectState {
		s.Fatal("Failed to set write protect state to ", state)
	}
	s.Log("FW write protect state has been successfully set to ", state)
}

func setFWWriteProtectState(ctx context.Context, h *firmware.Helper, enable bool) error {
	if enable {
		// enable SW WP before hardware WP
		if err := h.Servo.RunECCommand(ctx, "flashwp enable"); err != nil {
			return errors.Wrap(err, "failed to enable flashwp")
		}
		if err := h.Servo.SetFWWPState(ctx, servo.FWWPStateOn); err != nil {
			return errors.Wrap(err, "failed to disable firmware write protect")
		}
	} else {
		if err := h.Servo.SetFWWPState(ctx, servo.FWWPStateOff); err != nil {
			return errors.Wrap(err, "failed to disable firmware write protect")
		}
		// disable SW WP after hardware WP
		if err := h.Servo.RunECCommand(ctx, "flashwp disable"); err != nil {
			return errors.Wrap(err, "failed to disable flashwp")
		}
	}
	return nil
}

func verifyFWWriteProtectState(state string) (bool, error) {
	switch servo.FWWPStateValue(state) {
	case servo.FWWPStateOn:
		return true, nil
	case servo.FWWPStateOff:
		return false, nil
	default:
		return false, errors.New("current state is " + state)
	}
}
