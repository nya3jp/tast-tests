// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"strconv"
	"strings"
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
		Func:         ECPDRole,
		Desc:         "Verify USB-C/PD source role policy",
		LacrosStatus: testing.LacrosVariantUnneeded,
		Contacts:     []string{"cienet-firmware@cienet.corp-partner.google.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable"},
		Fixture:      fixture.NormalMode,
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.Lid()),
		Timeout:      20 * time.Minute,
	})
}

func ECPDRole(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to init servo: ", err)
	}
	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Failed to get config: ", err)
	}

	s.Log("Rebooting the DUT with hard reset")
	if err := h.Servo.SetPowerState(ctx, servo.PowerStateReset); err != nil {
		s.Fatal("Failed to EC reset DUT: ", err)
	}

	waitConnectCtx, cancelWaitConnect := context.WithTimeout(ctx, 8*time.Minute)
	defer cancelWaitConnect()

	if err := h.WaitConnect(waitConnectCtx); err != nil {
		s.Fatal("Failed to reconnect DUT: ", err)
	}

	// Parse the usb pd port list.
	usbcPorts, err := listUSBPdPorts(ctx, h)
	if err != nil {
		s.Fatal("Failed to list USB-C ports: ", err)
	}

	for _, step := range []struct {
		testAction   func(context.Context, *firmware.Helper) error
		expectStatus servo.USBPdDualRoleValue
	}{
		{
			testAction:   usbPdCloseLid,
			expectStatus: servo.USBPdDualRoleSink,
		},
		{
			testAction:   usbPdOpenLid,
			expectStatus: servo.USBPdDualRoleOn,
		},
		{
			testAction:   usbPdSuspend,
			expectStatus: servo.USBPdDualRoleOff,
		},
	} {
		if err := step.testAction(ctx, h); err != nil {
			s.Fatal("Action failed: ", err)
		}
		// Verify status of all usb-c port.
		for _, portID := range usbcPorts {
			if err := h.Servo.CheckUSBPdStatus(ctx, portID, step.expectStatus); err != nil {
				s.Fatal("Failed to check for USB PD: ", err)
			}
		}
	}
}

func listUSBPdPorts(ctx context.Context, h *firmware.Helper) ([]int, error) {
	bout, err := h.DUT.Conn().CommandContext(ctx, "ectool", "usbpdmuxinfo", "tsv").Output()
	if err != nil {
		return nil, errors.Wrap(err, "failed to run usbpdmuxinfo")
	}
	sout := strings.TrimSpace(string(bout))
	portNum := strings.Split(sout, "\n")
	if len(portNum) == 0 {
		return nil, errors.New("could not find any usb pd ports")
	}

	var usbPdPorts []int
	for _, port := range portNum {
		vals := strings.Split(port, "\t")
		p, err := strconv.Atoi(vals[0])
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse port ID")
		}
		usbPdPorts = append(usbPdPorts, p)
	}
	return usbPdPorts, nil
}

func usbPdCloseLid(ctx context.Context, h *firmware.Helper) error {
	if err := h.Servo.CloseLid(ctx); err != nil {
		return errors.Wrap(err, "failed to close lid")
	}
	testing.ContextLog(ctx, "Checking for G3 or S5 powerstate")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "G3", "S5"); err != nil {
		return errors.Wrap(err, "failed to get G3 or S5 powerstate")
	}
	return nil
}

func usbPdOpenLid(ctx context.Context, h *firmware.Helper) error {
	if err := h.Servo.OpenLid(ctx); err != nil {
		return errors.Wrap(err, "failed to open lid")
	}
	waitConnectCtx, cancelWaitConnect := context.WithTimeout(ctx, 2*time.Minute)
	defer cancelWaitConnect()

	if err := h.WaitConnect(waitConnectCtx); err != nil {
		return errors.Wrap(err, "failed to reconnect to DUT")
	}
	testing.ContextLog(ctx, "Checking for S0 powerstate")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0"); err != nil {
		return errors.Wrap(err, "failed to get power state at S0")
	}
	return nil
}

func usbPdSuspend(ctx context.Context, h *firmware.Helper) error {
	testing.ContextLog(ctx, "Suspending DUT")
	if err := h.DUT.Conn().CommandContext(ctx, "powerd_dbus_suspend").Start(); err != nil {
		return errors.Wrap(err, "failed to suspend DUT")
	}

	testing.ContextLog(ctx, "Checking for S0ix, S3, S5, or G3 powerstate")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0ix", "S3", "S5", "G3"); err != nil {
		return errors.Wrap(err, "failed to get power state at S0ix, S3, S5, or G3")
	}
	return nil
}
