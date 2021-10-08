// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ECWakeOnCharge,
		Desc:         "Checks that device will charge when EC is in a low-power mode, as a replacement for manual test 1.4.11",
		Contacts:     []string{"arthur.chuang@cienet.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_experimental"},
		Fixture:      fixture.DevModeGBB,
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
	})
}

func ECWakeOnCharge(ctx context.Context, s *testing.State) {

	// g3PollOptions sets the time to wait for DUT's power state to reach G3.
	g3PollOptions := testing.PollOptions{
		Timeout:  15 * time.Second,
		Interval: 1 * time.Second,
	}

	// getChargerPollOptions sets the time to retry the GetChargerAttached command.
	getChargerPollOptions := testing.PollOptions{
		Timeout:  10 * time.Second,
		Interval: 1 * time.Second,
	}

	// runECPollOptions sets the time to retry the RunECCommandGetOutput command.
	runECPollOptions := testing.PollOptions{
		Timeout:  10 * time.Second,
		Interval: 1 * time.Second,
	}

	d := s.DUT()

	h := s.FixtValue().(*fixture.Value).Helper

	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Failed to get config: ", err)
	}

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to init servo: ", err)
	}

	hasMicroOrC2D2, err := h.Servo.PreferDebugHeader(ctx)
	if err != nil {
		s.Fatal("PreferDebugHeader: ", err)
	}

	setPowerSupply := func(ctx context.Context, connectPower, hasHibernated bool) error {
		if connectPower {
			// Connect power supply.
			if err := h.SetDUTPower(ctx, true); err != nil {
				return errors.Wrap(err, "failed to connect charger")
			}

			// If DUT has hibernated before, reconnecting power supply will wake up the device.
			// Wait for DUT to reconnect.
			if hasHibernated {
				s.Log("Wait for DUT to power ON")
				waitConnectCtx, cancelWaitConnect := context.WithTimeout(ctx, 2*time.Minute)
				defer cancelWaitConnect()

				if err := d.WaitConnect(waitConnectCtx); err != nil {
					errors.Wrap(err, "failed to reconnect to DUT")
				}
			}

			// Verify EC console is responsive.
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				if _, err := h.Servo.RunECCommandGetOutput(ctx, "version", []string{`.`}); err != nil {
					return errors.Wrap(err, "EC is not responsive after reconnecting power supply to DUT")
				}
				return nil
			}, &runECPollOptions); err != nil {
				return errors.Wrap(err, "failed to wait for EC to become responsive")
			}
			s.Log("EC is responsive")

			// Verify that DUT is charging with power supply connected.
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				ok, err := h.Servo.GetChargerAttached(ctx)
				if err != nil {
					return errors.Wrap(err, "error checking whether charger is attached")
				}
				if !ok {
					return errors.New("DUT is not charging after connecting the power supply")
				}
				return nil
			}, &getChargerPollOptions); err != nil {
				return errors.Wrap(err, "failed to check for charger after connecting power")
			}
			s.Log("DUT is charging")

		} else {
			// Disconnect power supply.
			if err := h.SetDUTPower(ctx, false); err != nil {
				return errors.Wrap(err, "failed to remove charger")
			}

			// Verify that power supply was disconnected.
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				ok, err := h.Servo.GetChargerAttached(ctx)
				if err != nil {
					return errors.Wrap(err, "error checking whether charger is attached")
				}
				if ok {
					return errors.New("Charger is still attached")
				}
				return nil
			}, &getChargerPollOptions); err != nil {
				return errors.Wrap(err, "failed to check for charger after disconnecting power")
			}
			s.Log("Power supply was disconnected")

		}
		return nil
	}

	for _, tc := range []struct {
		lidOpen string
	}{
		{string(servo.LidOpenYes)},
		{string(servo.LidOpenNo)},
	} {
		s.Logf("-------------Test with lid open: %s-------------", tc.lidOpen)
		if err := h.Servo.SetStringAndCheck(ctx, servo.LidOpen, tc.lidOpen); err != nil {
			s.Fatal("Failed to set lid state: ", err)
		}

		var deviceHasHibernated bool

		s.Log("Stop AC Power")
		if err := setPowerSupply(ctx, false, deviceHasHibernated); err != nil {
			s.Fatal("Failed to stop power supply: ", err)
		}

		if tc.lidOpen == "yes" || hasMicroOrC2D2 {
			// Hibernate DUT
			s.Log("Put DUT in hibernation")
			if err = h.Servo.ECHibernate(ctx); err != nil {
				s.Fatal("Failed to hibernate: ", err)
			}
			deviceHasHibernated = true

		} else {
			// When using CCD, closed lid is emulated by an EC command, and hibernating EC will stop the emulation.
			// For this reason, we are using long power button press instead of setting EC to hibernate.
			s.Log("Long press on power key to put DUT into deep sleep mode")
			if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.Dur(h.Config.HoldPwrButtonPowerOff)); err != nil {
				s.Fatal("Failed to set a KeypressControl by servo: ", err)
			}

			s.Log("Wait for power state to become G3")
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

		s.Log("Reconnect AC power")
		if err := setPowerSupply(ctx, true, deviceHasHibernated); err != nil {
			s.Fatal("Failed to reconnect power supply: ", err)
		}

		// Verify DUT's lid current state remains the same as the initial state.
		lidStateFinal, err := h.Servo.LidOpenState(ctx)
		if err != nil {
			s.Fatal("Failed to check the final lid state: ", err)
		}
		if lidStateFinal != tc.lidOpen {
			s.Fatalf("DUT's lid_open state has changed from %s to %s", tc.lidOpen, lidStateFinal)
		}
	}
}
