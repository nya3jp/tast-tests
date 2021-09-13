// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/common/xmlrpc"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware/pre"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ECWakeOnCharge,
		Desc:         "Checks that device will charge when EC is in a low-power mode, as a replacement for manual test 1.4.11",
		Contacts:     []string{"arthur.chuang@cienet.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_experimental"},
		SoftwareDeps: pre.SoftwareDeps,
		Data:         pre.Data,
		Vars:         pre.Vars,
		ServiceDeps:  pre.ServiceDeps,
		Pre:          pre.DevModeGBB(),
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		Params: []testing.Param{
			{
				Name: "chrome_ec_device",
				Val:  true,
			},
			{
				Name: "non_chrome_ec_device",
				Val:  false,
			},
		},
	})
}

func ECWakeOnCharge(ctx context.Context, s *testing.State) {

	d := s.DUT()

	h := s.PreValue().(*pre.Value).Helper

	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Failed to get config: ", err)
	}

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to init servo: ", err)
	}

	for i := 0; i < 2; i++ {
		if i == 0 {
			s.Log("------------Test with lid open------------")
		}
		if i == 1 {
			s.Log("-----------Test with lid closed-----------")
			s.Log("Emulate closed lid")
			if err := h.Servo.SetStringAndCheck(ctx, servo.LidOpen, "no"); err != nil {
				s.Fatal("Failed to set lid state: ", err)
			}
		}

		// Check the initial lid state to compare at the end of test.
		lidStateInitial, err := h.Servo.LidOpenState(ctx)
		if err != nil {
			s.Fatal("Error checking the lid state: ", err)
		}
		s.Logf("Lid Open: %q", lidStateInitial)

		s.Log("Disconnect power supply by disabling charge-through")
		if err := h.Servo.SetPDRole(ctx, servo.PDRoleSnk); err != nil {
			s.Fatal("Error disabling charge-through: ", err)
		}

		s.Log("Verify that DUT's power supply was cut off")
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			ok, err := h.Servo.GetChargerAttached(ctx)
			if err != nil {
				return errors.Wrap(err, "error checking whether power is off")
			}
			if ok {
				return errors.New("Power remained on after disabling servo charger-through")
			}
			return nil
		}, &testing.PollOptions{Interval: 1 * time.Second, Timeout: 5 * time.Second}); err != nil {
			s.Fatal("Failed to cut DUT's power supply: ", err)
		}
		s.Log("Power supply is disconnected")

		chromeECDevice := s.Param().(bool)

		// For chrome EC devices, hibernate DUT by EC command.
		// Also, because emulation of closed lid would be compromised by hibernating EC,
		// hibernation is run only when lid is open.
		if chromeECDevice && i == 0 {
			s.Log("Put DUT in hibernation mode")
			if err := h.Servo.RunECCommand(ctx, "hibernate"); err != nil {
				s.Fatal("Failed to hibernate: ", err)
			}

			// Run an EC command to check if EC is alive. If EC not responsive, expect no return
			// for the query, and instead an error of type FaultError.
			s.Log("Verify EC console is non-responsive")
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				_, err := h.Servo.RunECCommandGetOutput(ctx, "version", []string{`.`})
				if err == nil {
					return errors.New("EC was still responsive after putting DUT in hibernation")
				}
				var errSend xmlrpc.FaultError
				if !errors.As(err, &errSend) {
					return errors.Wrap(err, "EC was still responsive after putting DUT in hibernation")
				}
				return nil
			}, &testing.PollOptions{Interval: 1 * time.Second, Timeout: 10 * time.Second}); err != nil {
				s.Fatal("Verification on non-reponsive EC failed: ", err)
			}
			s.Log("EC was non-responsive")
		}

		// For non-Chrome EC devices, hibernate DUT by long pressing on the power button.
		if !chromeECDevice {
			// Do a long power button press to hibernate DUT.
			// To be verified on arcada.sarien.
		}

		s.Log("Reconnect power supply by enabling charge-through")
		if err := h.Servo.SetPDRole(ctx, servo.PDRoleSrc); err != nil {
			s.Fatal("Error enabling charge-through: ", err)
		}

		// Wait for DUT to reboot and reconnect from hibernating with lid open.
		if i == 0 {
			s.Log("Wait for DUT to power ON")
			waitConnectCtx, cancelWaitConnect := context.WithTimeout(ctx, 2*time.Minute)
			defer cancelWaitConnect()

			if err := d.WaitConnect(waitConnectCtx); err != nil {
				s.Fatal("Failed to reconnect to DUT: ", err)
			}
		}

		s.Log("Verify EC console is responsive")
		if _, err := h.Servo.RunECCommandGetOutput(ctx, "version", []string{`.`}); err != nil {
			s.Fatal("EC is not responsive after reconnecting power supply to DUT")
		}
		s.Log("EC is responsive")

		s.Log("Verify DUT is charging")
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			isAttached, err := h.Servo.GetChargerAttached(ctx)
			if err != nil {
				return errors.Wrap(err, "error checking whether DUT is charging")
			}
			if !isAttached {
				return errors.New("DUT is not charging after re-connecting power supply")
			}
			return nil
		}, &testing.PollOptions{Interval: 1 * time.Second, Timeout: 5 * time.Second}); err != nil {
			s.Fatal("Charging failed: ", err)
		}
		s.Log("DUT is charging")

		s.Log("Verify DUT's lid state remains the same as the initial state")
		lidStateFinal, err := h.Servo.LidOpenState(ctx)
		if err != nil {
			s.Fatal("Error checking the lid state: ", err)
		}
		if lidStateFinal != lidStateInitial {
			s.Fatalf("DUT's lid_open state has changed from %q to %q", lidStateInitial, lidStateFinal)
		}
	}
}
