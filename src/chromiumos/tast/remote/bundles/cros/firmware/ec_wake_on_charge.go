// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
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

type debugInformation struct {
	servoConnectionType string
	servoType           string
	hasMicroOrC2D2      bool
	hasCCD              bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ECWakeOnCharge,
		Desc:         "Checks that device will charge when EC is in a low-power mode, as a replacement for manual test 1.4.11",
		LacrosStatus: testing.LacrosVariantUnneeded,
		Contacts:     []string{"arthur.chuang@cienet.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable", "firmware_bringup"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"board", "model"},
		Fixture:      fixture.NormalMode,
		ServiceDeps:  []string{"tast.cros.firmware.UtilsService"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.Battery()),
		Timeout:      30 * time.Minute,
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
		Timeout:  1 * time.Minute,
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
		s.Fatal("Failed to connect to servo: ", err)
	}

	// Increase timeout in getting response from ec uart.
	if err := h.Servo.SetString(ctx, "ec_uart_timeout", "10"); err != nil {
		s.Fatal("Failed to extend ec uart timeout: ", err)
	}
	defer func() {
		testing.ContextLog(ctx, "Restoring ec uart timeout to the default value of 3 seconds")
		if err := h.Servo.SetString(ctx, "ec_uart_timeout", "3"); err != nil {
			s.Fatal("Failed to restore default ec uart timeout: ", err)
		}
	}()

	// At start of test, save some information that may
	// be useful for debugging.
	var checkedInfo debugInformation
	if err := checkInformation(ctx, h, &checkedInfo); err != nil {
		s.Fatal("Unable to log information at start of test: ", err)
	}

	checkCCDTestlab := func(ctx context.Context) error {
		// Regular expressions.
		var (
			accessDenied          = `Access Denied`
			shortPPStart          = `\[\S+ PP start short\]`
			checkCCDTestlabEnable = `(` + accessDenied + `|` + shortPPStart + `)`
		)
		testlabStatus, err := h.Servo.GetString(ctx, servo.CR50Testlab)
		if err != nil {
			return errors.Wrap(err, "failed to get cr50_testlab")
		}
		if testlabStatus != string(servo.On) {
			s.Log("Checking if enabling testlab is possible")
			strings, err := h.Servo.RunCR50CommandGetOutput(ctx, "ccd testlab enable", []string{checkCCDTestlabEnable})
			if err != nil {
				s.Log("Unexpected error from 'ccd testlab enable': ", err)
			} else {
				s.Logf("Output from running 'ccd testlab enable': %s", strings[0][0])
			}
			// In case that 'ccd testlab enable' worked, sleep here regardless for the timeout to
			// happen on the power press.
			if err := testing.Sleep(ctx, 10*time.Second); err != nil {
				return errors.Wrap(err, "failed to sleep")
			}
		}
		return nil
	}

	openCCD := func(ctx context.Context) error {
		if checkedInfo.hasCCD {
			// Running 'ccd testlab open' would likely not work if it was not enabled.
			// But, if CCD was locked, testlab mode couldn't be enabled, and the
			// console output would print 'access denied'. For debugging purposes,
			// check here if enabling testlab mode is possible.
			// To-do: if 'ccd testlab open' fails, we might need to consider doing a
			// regular ap open.
			if val, err := h.Servo.GetString(ctx, servo.GSCCCDLevel); err != nil {
				return errors.Wrap(err, "failed to get gsc_ccd_level")
			} else if val != servo.Open {
				s.Logf("CCD is not open, got %q. Attempting to unlock", val)
				if err := checkCCDTestlab(ctx); err != nil {
					return err
				}
				if err := h.Servo.SetString(ctx, servo.CR50Testlab, servo.Open); err != nil {
					return errors.Wrap(err, "failed to unlock CCD")
				}
			}
		}
		return nil
	}
	setPowerSupply := func(ctx context.Context, connectPower, hasHibernated bool) error {
		if connectPower {
			// Connect power supply.
			if err := h.SetDUTPower(ctx, true); err != nil {
				return errors.Wrap(err, "failed to connect charger")
			}

			// On babytiger and babymega, leased from the lab,
			// it appeared that even though the servo command was
			// successful, the pd role didn't change. Check whether
			// this would also happen on the other DUTs. If it did,
			// exit from the test, and restore connection to the DUT
			// by the deferred cleanup.
			if err := testing.Sleep(ctx, 5*time.Second); err != nil {
				return errors.Wrap(err, "failed to sleep")
			}
			if checkedInfo.servoConnectionType == "type-c" {
				role, err := h.Servo.GetPDRole(ctx)
				if err != nil {
					return errors.Wrap(err, "failed to retrieve USB PD role for servo")
				}
				if role != servo.PDRoleSrc {
					return errors.Wrapf(err, "setting pd role to src, but got: %q", role)
				}
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
					return &retriableErr{E: errors.Wrap(err, "failed to reconnect to DUT")}
				}
				// Cr50 goes to sleep during hibernation, and when DUT wakes, CCD state might be locked.
				// Open CCD after waking DUT and before talking to the EC.
				if err := openCCD(ctx); err != nil {
					return err
				}
				// Sleep briefly to ensure that CCD has fully opened.
				if err := testing.Sleep(ctx, 1*time.Second); err != nil {
					return errors.Wrap(err, "failed to sleep")
				}
				// For debugging purposes, log the ccd state again for reference.
				if checkedInfo.hasCCD {
					valAfterOpenCCD, err := h.Servo.GetString(ctx, servo.GSCCCDLevel)
					if err != nil {
						return errors.Wrap(err, "failed to get gsc_ccd_level")
					}
					s.Logf("CCD state: %s", valAfterOpenCCD)
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

	// To prevent leaving DUT in an offline state, open lid or perform a cold reset at the end of test.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Minute)
	defer cancel()

	args := s.Param().(*testArgsForECWakeOnCharge)
	defer func(ctx context.Context, args *testArgsForECWakeOnCharge) {
		s.Log("Cold resetting DUT at the end of test")
		if err := h.Servo.SetPowerState(ctx, servo.PowerStateReset); err != nil {
			s.Fatal("Failed to cold reset DUT at the end of test: ", err)
		}
		s.Log("Waiting for reconnection to DUT")
		if err := h.WaitConnect(ctx); err != nil {
			s.Fatal("Unable to reconnect to DUT: ", err)
		}
		if args.hasLid {
			if err := h.Servo.OpenLid(ctx); err != nil {
				s.Fatal("Failed to set lid state: ", err)
			}
		}
		if args.formFactor == "convertible" {
			// Check for tablet mode angles, and if they are different
			// than the default values, restore them to default.
			cmd := firmware.NewECTool(s.DUT(), firmware.ECToolNameMain)
			tabletModeAngleInit, hysInit, err := cmd.SaveTabletModeAngles(ctx)
			if err != nil {
				s.Fatal("Failed to read tablet mode angles: ", err)
			} else if tabletModeAngleInit != "180" || hysInit != "20" {
				s.Log("Restoring ectool tablet mode angles to the default settings")
				if err := cmd.ForceTabletModeAngle(ctx, "180", "20"); err != nil {
					s.Fatal("Failed to restore tablet mode angles to the default settings: ", err)
				}
			}
		}
	}(cleanupCtx, args)

	// Monitor servo connection in the background, and attempt to reconnect if the connection drops.
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
				if err := testing.Sleep(ctx, time.Second); err != nil {
					s.Error("Failed to sleep while monitoring servo: ", err)
				}
				if _, err := h.Servo.Echo(ctx, "ping"); err != nil {
					s.Log("Failed to ping servo, reconnecting: ", err)
					err := h.ServoProxy.Reconnect(ctx)
					if err != nil {
						s.Error("Failed to connect to servo: ", err)
					}
				}
			}
		}
	}()
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

		if h.Config.Hibernate {
			if err := hibernateDUT(ctx, h, s.DUT(), checkedInfo.hasMicroOrC2D2, tc.lidOpen, args.formFactor, args.tabletModeOff); err != nil {
				s.Fatal("Failed to hibernate DUT: ", err)
			}
			deviceHasHibernated = true
		} else {
			// For DUTs that do not support the ec hibernation command, when lid is open, we could use
			// a long power button press instead to put DUT in deep sleep. But, when lid is closed without
			// log-in, power state will eventually reach G3.
			if tc.lidOpen == "yes" {
				s.Logf("Long pressing on power key for %s to put DUT into deep sleep mode", h.Config.HoldPwrButtonPowerOff)
				if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.Dur(h.Config.HoldPwrButtonPowerOff)); err != nil {
					s.Fatal("Failed to set a KeypressControl by servo: ", err)
				}
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
				switch tc.lidOpen {
				case "yes":
					// Power button is another wake up pin to wake DUT from hibernation.
					// If re-connecting charger fails in waking up DUT, try with a press
					// on power.
					retryCtx, cancelRetry := context.WithTimeout(ctx, 1*time.Minute)
					defer cancelRetry()
					if err := bootDUTIntoS0(retryCtx, h); err != nil {
						s.Fatal("Failed to reconnect to DUT: ", err)
					}
				case "no":
					// According to Stainless, when lid is closed, DUTs remain in G3
					// after waking up from hibernation with charger reconnected.
					s.Log("Checking if power state in G3 or S5")
					if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, 1*time.Minute, "G3", "S5"); err != nil {
						s.Fatal("Unable to get power state at G3 or S5: ", err)
					}
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
}

// checkTabletModeStatus checks whether ChromeOS is in tablet mode through the utils service.
func checkTabletModeStatus(ctx context.Context, h *firmware.Helper) (bool, error) {
	if err := h.RequireRPCUtils(ctx); err != nil {
		return false, errors.Wrap(err, "requiring RPC utils")
	}
	testing.ContextLog(ctx, "Sleeping for a few seconds before starting a new Chrome")
	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		return false, errors.Wrap(err, "failed to wait for a few seconds")
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

func bootDUTIntoS0(ctx context.Context, h *firmware.Helper) error {
	// Check if DUT is at G3. If DUT is in G3, use power button to boot it into S0.
	testing.ContextLog(ctx, "Checking if power states at G3 or S5")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, 1*time.Minute, "G3", "S5"); err != nil {
		return errors.Wrap(err, "unable to get power state at G3 or S5. DUT disconnected due to other reasons")
	}
	testing.ContextLogf(ctx, "Pressing power button for %s to wake DUT into S0 from G3 or S5", h.Config.HoldPwrButtonPowerOn)
	if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.Dur(h.Config.HoldPwrButtonPowerOn)); err != nil {
		return errors.Wrap(err, "failed to press power button")
	}
	testing.ContextLog(ctx, "Waiting for power state S0")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, 1*time.Minute, "S0"); err != nil {
		return errors.Wrap(err, "unable to get power state at S0")
	}
	return nil
}

func checkInformation(ctx context.Context, h *firmware.Helper, info *debugInformation) error {
	var wantInfo = []string{"hasMicroOrC2D2", "hasCCD", "servoType", "servoConnectionType"}
	for _, val := range wantInfo {
		var err error
		switch val {
		case "hasMicroOrC2D2":
			info.hasMicroOrC2D2, err = h.Servo.PreferDebugHeader(ctx)
		case "hasCCD":
			info.hasCCD, err = h.Servo.HasCCD(ctx)
		case "servoType":
			info.servoType, err = h.Servo.GetServoType(ctx)
		case "servoConnectionType":
			info.servoConnectionType, err = h.Servo.GetString(ctx, "root.dut_connection_type")
		}
		if err != nil {
			return errors.Wrapf(err, "failed to check for %s", val)
		}
	}
	testing.ContextLogf(ctx, "DUT has micro or c2d2: %t, has CCD: %t, servo type: %s, servo connection type: %s",
		info.hasMicroOrC2D2, info.hasCCD, info.servoType, info.servoConnectionType)
	return nil
}

func ensureClamshellMode(ctx context.Context, h *firmware.Helper, dut *dut.DUT, formFactor, tabletModeOff string) error {
	inTabletMode, err := checkTabletModeStatus(ctx, h)
	if err != nil {
		return errors.Wrap(err, "unable to check for DUT's tablet mode status")
	}
	if inTabletMode {
		testing.ContextLog(ctx, "DUT is in tablet mode. Attempting to turn tablet mode off")
		out, err := h.Servo.CheckAndRunTabletModeCommand(ctx, tabletModeOff)
		if err != nil {
			if formFactor == "convertible" {
				testing.ContextLogf(ctx, "Failed to run %s: %v. Attempting to set rotation angles with ectool instead", tabletModeOff, err)
				cmd := firmware.NewECTool(dut, firmware.ECToolNameMain)
				// Setting tabletModeAngle to 360 will force DUT into clamshell mode.
				if err := cmd.ForceTabletModeAngle(ctx, "360", "0"); err != nil {
					return errors.Wrap(err, "failed to set DUT in clamshell mode")
				}
				return nil
			}
			return errors.Wrapf(err, "failed to run %s", tabletModeOff)
		}
		// Allow some delay to ensure that DUT has completely transitioned out of tablet mode.
		if err := testing.Sleep(ctx, 3*time.Second); err != nil {
			return errors.Wrap(err, "failed to sleep")
		}
		testing.ContextLogf(ctx, "Tablet mode status: %s", out)
	}
	return nil
}

func hibernateDUT(ctx context.Context, h *firmware.Helper, dut *dut.DUT, hasMicroOrC2D2 bool, lidOpen, formFactor, tabletModeOff string) error {
	switch lidOpen {
	case "no":
		if hasMicroOrC2D2 {
			// In cases where lid is closed, and there's a servo_micro or C2D2 connection,
			// use console command to hibernate. Using keyboard presses might trigger DUT
			// to wake, and interrupt lid emulation.
			testing.ContextLog(ctx, "Putting DUT in hibernation with EC console command")
			if err := h.Servo.ECHibernate(ctx, servo.UseConsole); err != nil {
				return errors.Wrap(err, "failed to hibernate")
			}
			return nil
		}
		// Note: when lid is closed without log-in, power state transitions from S0 to S5,
		// and then eventually to G3, which would be equivalent to long pressing on power
		// to put DUT asleep.
		testing.ContextLog(ctx, "Waiting for power state to become G3 or S5")
		if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, 1*time.Minute, "G3", "S5"); err != nil {
			// Sometimes EC becomes unresponsive when lid is closed. But, for this test's
			// purposes, we're more concerned about whether DUT wakes when AC is re-attached.
			if strings.Contains(err.Error(), "No data was sent from the pty") ||
				strings.Contains(err.Error(), "Timed out waiting for interfaces to become available") {
				testing.ContextLog(ctx, "DUT appears to be completely offline. We're okay as long as reconnecting power resumes it")
			} else {
				return errors.Wrap(err, "failed to get powerstates at G3 or S5")
			}
		}
	case "yes":
		if formFactor == "convertible" || formFactor == "detachable" {
			// For convertibles and detachables, if DUT was left in tablet mode by previous tests,
			// key presses may be inactive. Always turn tablet mode off before using keyboard to hibernate.
			testing.ContextLogf(ctx, "DUT is a %s. Checking tablet mode status before using keyboard to hibernate", formFactor)
			if err := ensureClamshellMode(ctx, h, dut, formFactor, tabletModeOff); err != nil {
				return err
			}
		}
		testing.ContextLog(ctx, "Putting DUT in hibernation with key presses")
		if err := h.Servo.ECHibernate(ctx, servo.UseKeyboard); err != nil {
			testing.ContextLogf(ctx, "Failed to hibernate: %v. Retry with using EC console command to hibernate", err)
			if err := h.Servo.ECHibernate(ctx, servo.UseConsole); err != nil {
				return errors.Wrap(err, "failed to hibernate")
			}
		}
		h.DisconnectDUT(ctx)
	}
	return nil
}
