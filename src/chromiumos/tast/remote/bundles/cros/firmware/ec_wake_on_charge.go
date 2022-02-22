// Copyright 2021 The Chromium OS Authors. All rights reserved.
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
		Func:         ECWakeOnCharge,
		Desc:         "Checks that device will charge when EC is in a low-power mode, as a replacement for manual test 1.4.11",
		Contacts:     []string{"arthur.chuang@cienet.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable", "firmware_bringup"},
		Vars:         []string{"board", "model"},
		Fixture:      fixture.NormalMode,
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.Battery()),
		Params: []testing.Param{{
			Name:              "device_without_lid",
			ExtraHardwareDeps: hwdep.D(hwdep.FormFactor(hwdep.Chromeslate)),
			Val:               false,
		}, {
			ExtraHardwareDeps: hwdep.D(hwdep.Lid()),
			Val:               true,
		}},
	})
}

func ECWakeOnCharge(ctx context.Context, s *testing.State) {
	// getChargerPollOptions sets the time to retry the GetChargerAttached command.
	getChargerPollOptions := testing.PollOptions{
		Timeout:  30 * time.Second,
		Interval: 1 * time.Second,
	}

	// runECPollOptions sets the time to retry the RunECCommandGetOutput command.
	runECPollOptions := testing.PollOptions{
		Timeout:  30 * time.Second,
		Interval: 1 * time.Second,
	}

	h := s.FixtValue().(*fixture.Value).Helper

	board, _ := s.Var("board")
	model, _ := s.Var("model")
	h.OverridePlatform(ctx, board, model)

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
				s.Log("Waiting for DUT to power ON")
				waitConnectCtx, cancelWaitConnect := context.WithTimeout(ctx, 2*time.Minute)
				defer cancelWaitConnect()

				var opts []firmware.WaitConnectOption
				opts = append(opts, firmware.FromHibernation)
				if err := h.WaitConnect(waitConnectCtx, opts...); err != nil {
					errors.Wrap(err, "failed to reconnect to DUT")
				}
				// Cr50 goes to sleep during hibernation, and when DUT wakes, CCD state might be locked.
				// Open CCD after waking DUT and before talking to the EC.
				if hasCCD, err := h.Servo.HasCCD(ctx); err != nil {
					s.Fatal("While checking if servo has a CCD connection: ", err)
				} else if hasCCD {
					if val, err := h.Servo.GetString(ctx, servo.CR50CCDLevel); err != nil {
						s.Fatal("Failed to get cr50_ccd_level: ", err)
					} else if val != servo.Open {
						s.Logf("CCD is not open, got %q. Attempting to unlock", val)
						if err := h.Servo.SetString(ctx, servo.CR50Testlab, servo.Open); err != nil {
							s.Fatal("Failed to unlock CCD: ", err)
						}
					}
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
					return errors.New("charger is still attached - use Servo V4 Type-C or supply RPM vars")
				}
				return nil
			}, &getChargerPollOptions); err != nil {
				return errors.Wrap(err, "failed to check for charger after disconnecting power")
			}
			s.Log("Power supply was disconnected")
		}
		return nil
	}

	done := make(chan struct{})
	defer func() {
		done <- struct{}{}
	}()

	go func() {
	monitorServo:
		for {
			select {
			case <-done:
				break monitorServo
			default:
				if err = testing.Sleep(ctx, time.Second); err != nil {
					s.Error("Failed to sleep: ", err)
				}
				if _, err = h.Servo.Echo(ctx, "ping"); err != nil {
					s.Log("Failed to ping servo, reconnecting: ", err)
					err = h.ServoProxy.Reconnect(ctx)
					if err != nil {
						s.Error("Failed to reconnect to servo: ", err)
					}
				}
			}
		}
	}()

	deviceHasLid := s.Param().(bool)
	for _, tc := range []struct {
		lidOpen string
	}{
		{string(servo.LidOpenYes)},
		{string(servo.LidOpenNo)},
	} {
		// Only repeat the test in lid closed when device has a lid.
		if tc.lidOpen == "no" && deviceHasLid == false {
			break
		}

		// Skip setting the lid state for DUTs that don't have a lid, i.e. Chromeslates.
		if deviceHasLid {
			s.Logf("-------------Test with lid open: %s-------------", tc.lidOpen)
			if err := h.Servo.SetStringAndCheck(ctx, servo.LidOpen, tc.lidOpen); err != nil {
				s.Fatal("Failed to set lid state: ", err)
			}
		}

		// Delay for some time to ensure lid was properly closed, or opened.
		if err := testing.Sleep(ctx, 3*time.Second); err != nil {
			s.Fatal("Failed to sleep: ", err)
		}

		var deviceHasHibernated bool
		s.Log("Stopping AC Power")
		if err := setPowerSupply(ctx, false, deviceHasHibernated); err != nil {
			s.Fatal("Failed to stop power supply: ", err)
		}

		if (tc.lidOpen == "yes" || hasMicroOrC2D2) && h.Config.Hibernate {
			// Hibernate DUT
			s.Log("Putting DUT in hibernation")
			if tc.lidOpen == "no" {
				// In cases where lid is closed, and there's a servo_micro or C2D2 connection,
				// use console command to hibernate. Using keyboard presses might trigger DUT
				// to wake, as well as interrupt lid emulation.
				if err = h.Servo.ECHibernate(ctx, servo.UseConsole); err != nil {
					s.Fatal("Failed to hibernate: ", err)
				}
			} else {
				if err = h.Servo.ECHibernate(ctx, servo.UseKeyboard); err != nil {
					s.Fatal("Failed to hibernate: ", err)
				}
			}
			h.DisconnectDUT(ctx)
			deviceHasHibernated = true
		} else if tc.lidOpen == "no" {
			// Note: when lid is closed without log-in, power state transitions from S0 to S5,
			// and then eventually to G3, which would be equivalent to long pressing on power
			// to put DUT asleep.
			s.Log("Waiting for power state to become G3 or S5")
			if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "G3", "S5"); err != nil {
				s.Fatal("Failed to get powerstates at G3 or S5: ", err)
			}
		} else {
			// For DUTs that do not support the ec hibernation command, we would use
			// a long power button press instead.
			s.Log("Long pressing on power key to put DUT into deep sleep mode")
			if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.Dur(h.Config.HoldPwrButtonPowerOff)); err != nil {
				s.Fatal("Failed to set a KeypressControl by servo: ", err)
			}

			s.Log("Waiting for power state to become G3 or S3")
			if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "G3", "S3"); err != nil {
				s.Fatal("Failed to get powerstates at G3 or S3: ", err)
			}
		}

		s.Log("Reconnecting AC")
		if err := setPowerSupply(ctx, true, deviceHasHibernated); err != nil {
			s.Fatal("Failed to reconnect power supply: ", err)
		}

		if deviceHasLid {
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

	if deviceHasLid {
		if err := h.Servo.OpenLid(ctx); err != nil {
			s.Fatal("Failed to set lid state: ", err)
		}
	}
}
