// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type testArgsForECWakeOnCharge struct {
	formFactor    string
	tabletModeOff string
	hasLid        bool
}

type retriableErr struct {
	*errors.E
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ECWakeOnCharge,
		Desc:         "Checks that device will charge when EC is in a low-power mode, as a replacement for manual test 1.4.11",
		LacrosStatus: testing.LacrosVariantUnknown,
		Contacts:     []string{"arthur.chuang@cienet.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable", "firmware_bringup"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"board", "model"},
		Fixture:      fixture.NormalMode,
		ServiceDeps:  []string{"tast.cros.firmware.UtilsService"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.Battery()),
		Timeout:      15 * time.Minute,
		Params: []testing.Param{{
			Name:              "chromeslate",
			ExtraHardwareDeps: hwdep.D(hwdep.FormFactor(hwdep.Chromeslate)),
			Val: &testArgsForECWakeOnCharge{
				formFactor: "chromeslate",
				hasLid:     false,
			},
		}, {
			Name:              "convertible",
			ExtraHardwareDeps: hwdep.D(hwdep.FormFactor(hwdep.Convertible)),
			Val: &testArgsForECWakeOnCharge{
				formFactor:    "convertible",
				tabletModeOff: "tabletmode off",
				hasLid:        true,
			},
		}, {
			Name:              "detachable",
			ExtraHardwareDeps: hwdep.D(hwdep.FormFactor(hwdep.Detachable)),
			Val: &testArgsForECWakeOnCharge{
				formFactor:    "detachable",
				tabletModeOff: "basestate attach",
				hasLid:        true,
			},
		}, {
			ExtraHardwareDeps: hwdep.D(hwdep.FormFactor(hwdep.Clamshell)),
			Val: &testArgsForECWakeOnCharge{
				formFactor: "clamshell",
				hasLid:     true,
			},
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

	openCCD := func(ctx context.Context) error {
		if hasCCD, err := h.Servo.HasCCD(ctx); err != nil {
			return errors.Wrap(err, "while checking if servo has a CCD connection")
		} else if hasCCD {
			if val, err := h.Servo.GetString(ctx, servo.CR50CCDLevel); err != nil {
				return errors.Wrap(err, "failed to get cr50_ccd_level")
			} else if val != servo.Open {
				s.Logf("CCD is not open, got %q. Attempting to unlock", val)
				if err := h.Servo.SetString(ctx, servo.CR50Testlab, servo.Open); err != nil {
					return errors.Wrap(err, "failed to unlock CCD")
				}
			}
		}
		return nil
	}
	setPowerSupply := func(ctx context.Context, connectPower, hasHibernated bool) error {
		// For debugging purposes, explicitly log servo connection type. Servo might report that it has
		// control over pd role even for Type-A. Logging this information would clarify on whether other
		// failures occur because the pd role wasn't switched in the first place with a type A connection.
		if connectionType, err := h.Servo.GetString(ctx, "root.dut_connection_type"); err != nil {
			s.Log("Unable to read servo connection type: ", err)
		} else {
			s.Logf("Servo connection type: %s", connectionType)
		}
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
					return &retriableErr{E: errors.New("failed to reconnect to DUT")}
				}
				// Cr50 goes to sleep during hibernation, and when DUT wakes, CCD state might be locked.
				// Open CCD after waking DUT and before talking to the EC.
				if err := openCCD(ctx); err != nil {
					return err
				}
			}

			// Sleep briefly to ensure that CCD has fully opened.
			if err := testing.Sleep(ctx, 1*time.Second); err != nil {
				return errors.Wrap(err, "failed to sleep")
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
					s.Error("Failed to sleep while monitoring servo: ", err)
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

	args := s.Param().(*testArgsForECWakeOnCharge)
	for _, tc := range []struct {
		lidOpen string
	}{
		{string(servo.LidOpenYes)},
		{string(servo.LidOpenNo)},
	} {
		// Only repeat the test in lid closed when device has a lid.
		if tc.lidOpen == "no" && args.hasLid == false {
			break
		}

		var deviceHasHibernated bool
		s.Log("Stopping AC Power")
		if err := setPowerSupply(ctx, false, deviceHasHibernated); err != nil {
			s.Fatal("Failed to stop power supply: ", err)
		}

		// When power is cut, there may be a temporary disconnection between servo and DUT,
		// and the duration may vary from model to model. Delaying for some time here seems
		// to be helpful in preventing EC errors.
		s.Log("Sleeping for 15 seconds")
		if err := testing.Sleep(ctx, 15*time.Second); err != nil {
			s.Fatal("Failed to sleep: ", err)
		}

		// Skip setting the lid state for DUTs that don't have a lid, i.e. Chromeslates.
		if args.hasLid {
			s.Logf("-------------Test with lid open: %s-------------", tc.lidOpen)
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				if err := h.Servo.SetStringAndCheck(ctx, servo.LidOpen, tc.lidOpen); err != nil {
					// This error may be temporary.
					if strings.Contains(err.Error(), "No data was sent from the pty") ||
						strings.Contains(err.Error(), "Timed out waiting for interfaces to become available") {
						return err
					}
					return testing.PollBreak(err)
				}
				return nil
			}, &testing.PollOptions{Timeout: 1 * time.Minute, Interval: 1 * time.Second}); err != nil {
				s.Fatal("Failed to set lid state: ", err)
			}
			// There's a chance that CCD would close when lid closed.
			// Open CCD to prevent errors from running EC commands.
			if tc.lidOpen == "no" {
				if err := openCCD(ctx); err != nil {
					s.Fatal("Failed to open CCD after closing lid: ", err)
				}
			}
		}

		// Between cutting power and sending DUT into hibernation, waiting
		// for some delay seems to help with preventing servo exit.
		s.Log("Sleeping for 30 seconds")
		if err := testing.Sleep(ctx, 30*time.Second); err != nil {
			s.Fatal("Failed to sleep: ", err)
		}

		if (tc.lidOpen == "yes" || hasMicroOrC2D2) && h.Config.Hibernate {
			// Hibernate DUT
			if tc.lidOpen == "no" {
				// In cases where lid is closed, and there's a servo_micro or C2D2 connection,
				// use console command to hibernate. Using keyboard presses might trigger DUT
				// to wake, as well as interrupt lid emulation.
				s.Log("Putting DUT in hibernation with EC console command")
				if err = h.Servo.ECHibernate(ctx, servo.UseConsole); err != nil {
					s.Fatal("Failed to hibernate: ", err)
				}
			} else {
				if args.formFactor == "convertible" || args.formFactor == "detachable" {
					// For convertibles and detachables, if DUT was left in tablet mode by previous tests,
					// key presses may be inactive. Always turn tablet mode off before using keyboard to hibernate.
					s.Logf("DUT is a %s. Checking tablet mode status before using keyboard to hibernate", args.formFactor)
					if inTabletMode, err := checkTabletModeStatus(ctx, h); err != nil {
						s.Fatal("Unable to check DUT's tablet mode status: ", err)
					} else if inTabletMode {
						s.Log("DUT is in tablet mode. Attempting to turn tablet mode off")
						out, err := h.Servo.CheckAndRunTabletModeCommand(ctx, args.tabletModeOff)
						if err != nil {
							if args.formFactor == "convertible" {
								s.Logf("Failed to run %s: %v. Attempting to set rotation angles with ectool instead", args.tabletModeOff, err)
								cmd := firmware.NewECTool(s.DUT(), firmware.ECToolNameMain)
								tabletModeAngleInit, hysInit, err := saveAnglesAndForceClamshell(ctx, cmd)
								if err != nil {
									s.Fatal("Failed to force DUT into clamshell mode: ", err)
								}
								defer func(cmd *firmware.ECTool) {
									s.Logf("Restoring DUT's tablet mode angles to the original settings: lid_angle=%s, hys=%s", tabletModeAngleInit, hysInit)
									if err := cmd.ForceTabletModeAngle(ctx, tabletModeAngleInit, hysInit); err != nil {
										s.Fatal("Failed to restore tablet mode angle to the initial angles: ", err)
									}
								}(cmd)
							} else {
								s.Fatalf("Failed to run %s: %v", args.tabletModeOff, err)
							}
						}
						s.Logf("Tablet mode status: %s", out)
						// Allow some delay to ensure that DUT has completely transitioned out of tablet mode.
						if err := testing.Sleep(ctx, 3*time.Second); err != nil {
							s.Fatal("Failed to sleep: ", err)
						}
					}
				}
				s.Log("Putting DUT in hibernation with key presses")
				if err = h.Servo.ECHibernate(ctx, servo.UseKeyboard); err != nil {
					s.Logf("Failed to hibernate: %v. Retry with using EC console command to hibernate", err)
					if err = h.Servo.ECHibernate(ctx, servo.UseConsole); err != nil {
						s.Fatal("Failed to hibernate: ", err)
					}
				}
			}
			h.DisconnectDUT(ctx)
			deviceHasHibernated = true
		} else if tc.lidOpen == "no" {
			// Note: when lid is closed without log-in, power state transitions from S0 to S5,
			// and then eventually to G3, which would be equivalent to long pressing on power
			// to put DUT asleep.
			s.Log("Waiting for power state to become G3 or S5")
			if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, 1*time.Minute, "G3", "S5"); err != nil {
				// On Hayato/Asurada, sometimes EC becomes unresponsive when lid is closed. But, for this test's
				// purposes, we're more concerned about whether DUT awakes when AC is applied.
				if strings.Contains(err.Error(), "No data was sent from the pty") ||
					strings.Contains(err.Error(), "Timed out waiting for interfaces to become available") {
					s.Log("DUT appears to be completely offline. We're okay as long as reconnecting power resumes it")
				} else {
					s.Fatal("Failed to get powerstates at G3 or S5: ", err)
				}
			}
		} else {
			// For DUTs that do not support the ec hibernation command, we would use
			// a long power button press instead.
			s.Log("Long pressing on power key to put DUT into deep sleep mode")
			if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.Dur(h.Config.HoldPwrButtonPowerOff)); err != nil {
				s.Fatal("Failed to set a KeypressControl by servo: ", err)
			}

			s.Log("Waiting for power state to become G3 or S3")
			if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, 1*time.Minute, "G3", "S3"); err != nil {
				s.Fatal("Failed to get powerstates at G3 or S3: ", err)
			}
		}

		s.Log("Reconnecting AC")
		if err := setPowerSupply(ctx, true, deviceHasHibernated); err != nil {
			if _, ok := err.(*retriableErr); ok {
				s.Log("Retriable error: ", err.(*retriableErr))
				retryCtx, cancelRetry := context.WithTimeout(ctx, 1*time.Minute)
				defer cancelRetry()
				if err := bootDUTIntoS0(retryCtx, h); err != nil {
					s.Fatal("Unable to reconnect to DUT: ", err)
				}
				// Delay for some time to ensure that DUT has fully settled down.
				s.Log("Sleeping for 30 seconds")
				if err := testing.Sleep(ctx, 30*time.Second); err != nil {
					s.Fatal("Failed to sleep for 30 seconds: ", err)
				}
			} else {
				s.Fatal("Failed to reconnect power supply: ", err)
			}
		}

		// Stainless results showed that on Nami DUTs, when tested with lid closed,
		// lid's state changed after waking up from hibernation. More research
		// might be needed, but for now skip on checking lid state for Nami devices.
		if args.hasLid && h.Config.Platform != "nami" {
			// Verify DUT's lid current state remains the same as the initial state.
			s.Log("Checking lid state hasn't changed")
			lidStateFinal, err := h.Servo.LidOpenState(ctx)
			if err != nil {
				s.Fatal("Failed to check the final lid state: ", err)
			}
			if lidStateFinal != tc.lidOpen {
				s.Fatalf("DUT's lid_open state has changed from %s to %s", tc.lidOpen, lidStateFinal)
			}
		}
	}

	if args.hasLid {
		if err := h.Servo.OpenLid(ctx); err != nil {
			s.Fatal("Failed to set lid state: ", err)
		}
		s.Log("Waiting for S0 powerstate")
		if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, 1*time.Minute, "S0"); err != nil {
			s.Fatal("Failed to wait for S0 powerstate: ", err)
		}
		if err := h.WaitConnect(ctx); err != nil {
			s.Fatal("Failed to reconnect to DUT after opening lid: ", err)
		}
	}
}

// checkTabletModeStatus checks whether ChromeOS is in tablet mode through the utils service.
func checkTabletModeStatus(ctx context.Context, h *firmware.Helper) (bool, error) {
	if err := h.RequireRPCUtils(ctx); err != nil {
		return false, errors.Wrap(err, "requiring RPC utils")
	}
	if _, err := h.RPCUtils.NewChrome(ctx, &empty.Empty{}); err != nil {
		return false, errors.Wrap(err, "failed to create instance of chrome")
	}
	defer h.RPCUtils.CloseChrome(ctx, &empty.Empty{})
	res, err := h.RPCUtils.EvalTabletMode(ctx, &empty.Empty{})
	if err != nil {
		return false, err
	}
	return res.TabletModeEnabled, nil
}

// saveAnglesAndForceClamshell saves the initial tablet mode angles from ectool, and then
// force sets them into the clamshell mode angles.
func saveAnglesAndForceClamshell(ctx context.Context, cmd *firmware.ECTool) (string, string, error) {
	// Save initial tablet mode angle settings to restore at the end of test.
	tabletModeAngleInit, hysInit, err := cmd.SaveTabletModeAngles(ctx)
	if err != nil {
		return "", "", errors.Wrap(err, "failed to save initial tablet mode angles")
	}
	// Setting tabletModeAngle to 360 will force DUT into clamshell mode.
	if err := cmd.ForceTabletModeAngle(ctx, "360", "0"); err != nil {
		return "", "", errors.Wrap(err, "failed to set clamshell mode angles")
	}
	return tabletModeAngleInit, hysInit, nil
}

func bootDUTIntoS0(ctx context.Context, h *firmware.Helper) error {
	// Check if DUT is at G3. If DUT is in G3, use power button to boot it into S0.
	testing.ContextLog(ctx, "Checking if power states at G3 or S5")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, 1*time.Minute, "G3", "S5"); err != nil {
		return errors.Wrap(err, "unable to get power state at G3 or S5. DUT disconnected due to other reasons")
	}
	testing.ContextLog(ctx, "Pressing power button to wake DUT into S0 from G3 or S5")
	if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurTab); err != nil {
		return errors.Wrap(err, "failed to press power button")
	}
	testing.ContextLog(ctx, "Waiting for power state S0")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, 1*time.Minute, "S0"); err != nil {
		return errors.Wrap(err, "unable to get power state at S0")
	}
	return nil
}
