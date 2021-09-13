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

// ecWakeOnChargeTestParams defines the params for a signle test-case.
// supportECHibernate defines whether the platform can perform hibernatation by an EC command.
type ecWakeOnChargeTestParams struct {
	supportECHibernate bool
}

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
		Params: []testing.Param{
			{
				Name:              "chrome_ec_device",
				ExtraHardwareDeps: hwdep.D(hwdep.ChromeEC()),
				Val: ecWakeOnChargeTestParams{
					supportECHibernate: true,
				},
				Timeout: 20 * time.Minute,
			},
			{
				Name: "non_chrome_ec_device",
				Val: ecWakeOnChargeTestParams{
					supportECHibernate: false,
				},
				Timeout: 20 * time.Minute,
			},
		},
	})
}

func ECWakeOnCharge(ctx context.Context, s *testing.State) {

	// g3PollOptions sets the time to wait for DUT's power state to reach G3.
	g3PollOptions := testing.PollOptions{
		Timeout:  15 * time.Second,
		Interval: 1 * time.Second,
	}

	// getChargerPollOptions sets the time to wait for the GetChargerAttached command to return true/false.
	getChargerPollOptions := testing.PollOptions{
		Timeout:  5 * time.Second,
		Interval: 1 * time.Second,
	}

	// runECPollOptions sets the time to retry the RunECCommandGetOutput command.
	runECPollOptions := testing.PollOptions{
		Timeout:  10 * time.Second,
		Interval: 1 * time.Second,
	}

	param := s.Param().(ecWakeOnChargeTestParams)

	d := s.DUT()

	h := s.PreValue().(*pre.Value).Helper

	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Failed to get config: ", err)
	}

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to init servo: ", err)
	}

	for _, tc := range []struct {
		lidOpen string
	}{
		{string(servo.LidOpenYes)},
		{string(servo.LidOpenNo)},
	} {
		s.Logf("-------------Test with lid open: %q-------------", string(tc.lidOpen))
		if err := h.Servo.SetStringAndCheck(ctx, servo.LidOpen, tc.lidOpen); err != nil {
			s.Fatal("Failed to set lid state: ", err)
		}

		// Check the initial lid state to compare at the end of test.
		lidStateInitial, err := h.Servo.LidOpenState(ctx)
		if err != nil {
			s.Fatal("Error checking the lid state: ", err)
		}
		s.Logf("Lid Open: %q", lidStateInitial)

		// Disconnect AC power
		s.Log("Disconnect power supply")
		if err := h.SetDUTPower(ctx, false); err != nil {
			s.Fatal("Failed to disable charge-through: ", err)
		}

		// Verify that DUT's power supply was cut off
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			ok, err := h.Servo.GetChargerAttached(ctx)
			if err != nil {
				return errors.Wrap(err, "error checking whether power is off")
			}
			if ok {
				return errors.New("Power remained on after disabling servo charger-through")
			}
			return nil
		}, &getChargerPollOptions); err != nil {
			s.Fatal("Failed to cut DUT's power supply: ", err)
		}
		s.Log("Power supply was disconnected")

		hasMicroOrC2D2, err := h.Servo.PreferDebugHeader(ctx)
		if err != nil {
			s.Fatal("PreferDebugHeader: ", err)
		}

		if param.supportECHibernate {
			if tc.lidOpen == "yes" || hasMicroOrC2D2 {

				// Hibernate DUT
				s.Log("Put DUT in hibernation")
				if err = h.Servo.ECHibernate(ctx); err != nil {
					s.Fatal("Failed to hibernate: ", err)
				}

				// Verify EC console is non-responsive
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
				}, &runECPollOptions); err != nil {
					s.Fatal("Failed to wait for EC to become non-responsive: ", err)
				}
				s.Log("EC was non-responsive")
			}
		}

		if !param.supportECHibernate {
			s.Log("--------------Pending verification: do a long power button press to hibernate DUT.--------------")
		}

		// Reconnect AC power
		s.Log("Reconnect power supply")
		if err := h.SetDUTPower(ctx, true); err != nil {
			s.Fatal("Failed to enable charge-through: ", err)
		}

		if tc.lidOpen == "yes" {
			s.Log("Wait for DUT to power ON")
			waitConnectCtx, cancelWaitConnect := context.WithTimeout(ctx, 2*time.Minute)
			defer cancelWaitConnect()

			if err := d.WaitConnect(waitConnectCtx); err != nil {
				s.Fatal("Failed to reconnect to DUT: ", err)
			}
		}

		// Verify EC console is responsive
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if _, err := h.Servo.RunECCommandGetOutput(ctx, "version", []string{`.`}); err != nil {
				return errors.Wrap(err, "EC is not responsive after reconnecting power supply to DUT")
			}
			return nil
		}, &runECPollOptions); err != nil {
			s.Fatal("Failed to wait for EC to become responsive: ", err)
		}
		s.Log("EC is responsive")

		// Verify DUT is charging
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			isAttached, err := h.Servo.GetChargerAttached(ctx)
			if err != nil {
				return errors.Wrap(err, "error checking whether DUT is charging")
			}
			if !isAttached {
				return errors.New("DUT is not charging after re-connecting power supply")
			}
			return nil
		}, &getChargerPollOptions); err != nil {
			s.Fatal("Charging failed: ", err)
		}
		s.Log("DUT is charging")

		// Verify DUT's lid state remains the same as the initial state
		lidStateFinal, err := h.Servo.LidOpenState(ctx)
		if err != nil {
			s.Fatal("Error checking the lid state: ", err)
		}
		if lidStateFinal != lidStateInitial {
			s.Fatalf("DUT's lid_open state has changed from %q to %q", lidStateInitial, lidStateFinal)
		}

		if tc.lidOpen == "no" {
			s.Log("Check that power state is G3")
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				state, err := h.Servo.GetECSystemPowerState(ctx)
				if err != nil {
					return testing.PollBreak(errors.Wrap(err, "failed to get power state"))
				}
				if state != "G3" {
					return errors.New("power state is " + state)
				}
				return nil
			}, &g3PollOptions); err != nil {
				s.Fatal("Failed to wait power state to be G3: ", err)
			}
		}

	}
}
