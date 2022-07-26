// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	pb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const (
	// Sleep time is set to 60 seconds due to SMP batteries requirement.
	sleepDuration = 60 * time.Second
	// Expected error messages when the EC is unresponsive.
	errmsg1 = "Timed out waiting for interfaces to become available"
	errmsg2 = "No data was sent from the pty"
	errmsg3 = "EC: Timeout waiting for response."
)

type reconnectErr struct {
	*errors.E
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         BootBatteryCutoff,
		Desc:         "Verify if system can boot after battery cutoff",
		Contacts:     []string{"cienet-firmware@cienet.corp-partner.google.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable"},
		Fixture:      fixture.NormalMode,
		ServiceDeps:  []string{"tast.cros.firmware.BiosService"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.Battery()),
		Params: []testing.Param{{
			Name:              "chromeslate",
			ExtraHardwareDeps: hwdep.D(hwdep.FormFactor(hwdep.Chromeslate)),
			Val:               true,
		}, {
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnFormFactor(hwdep.Chromeslate)),
			Val:               false,
		}},
		Timeout: 10 * time.Minute,
	})
}

func BootBatteryCutoff(ctx context.Context, s *testing.State) {
	ffIsChromeslate := s.Param().(bool)

	h := s.FixtValue().(*fixture.Value).Helper

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to init servo: ", err)
	}

	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Failed to get config: ", err)
	}

	// For debugging purposes, log servo and dut connection type.
	servoType, err := h.Servo.GetServoType(ctx)
	if err != nil {
		s.Fatal("Failed to find servo type: ", err)
	}
	s.Logf("Servo type: %s", servoType)

	dutConnType, err := h.Servo.GetDUTConnectionType(ctx)
	if err != nil {
		s.Fatal("Failed to find dut connection type: ", err)
	}
	s.Logf("DUT connection type: %s", dutConnType)

	if err := h.RequireBiosServiceClient(ctx); err != nil {
		s.Fatal("Failed to get bios service: ", err)
	}

	hasMicroOrC2D2, err := h.Servo.PreferDebugHeader(ctx)
	if err != nil {
		s.Fatal("PreferDebugHeader: ", err)
	}

	// This function will disconnect the charger, send the command Batterycutoff, and wait for 60 seconds.
	sendingBatterryCutoff := func(ctx context.Context) error {
		// Disconnect Charger.
		s.Log("Stopping power supply")
		if err := h.SetDUTPower(ctx, false); err != nil {
			return errors.Wrap(err, "failed to remove charger")
		}

		// Verify that charging stopped before sending battery cutoff.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			chargerAttached, err := h.Servo.GetChargerAttached(ctx)
			if err != nil {
				return errors.Wrap(err, "error checking whether charger is attached")
			}
			if chargerAttached {
				return errors.New("charger was not removed")
			}
			return nil
		}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: 1 * time.Second}); err != nil {
			return errors.Wrap(err, "failed to check for charger after stopping power suply")
		}
		s.Log("Charger is removed")

		// Remove CCD watchdog for servod not to close when power supply is stopped after sending batterycutoff command.
		s.Log("Disabling CCD watchdog")
		if err := h.Servo.WatchdogRemove(ctx, servo.WatchdogCCD); err != nil {
			s.Fatal("Failed to remove CCD watchdog: ", err)
		}

		// Send batterycutoff command.
		s.Log("Sending batterycutoff command")
		if err := s.DUT().Conn().CommandContext(ctx, "ectool", "batterycutoff").Start(); err != nil {
			return errors.Wrap(err, "failed to send batterycutoff command")
		}

		// Verify the DUT becomes unresponsive.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			_, err := h.Servo.RunECCommandGetOutput(ctx, "version", []string{`.`})
			if err == nil {
				return errors.Wrap(err, "EC is still active after Batterycutoff")
			}
			if !strings.Contains(err.Error(), errmsg1) && !strings.Contains(err.Error(), errmsg2) && !strings.Contains(err.Error(), errmsg3) {
				return errors.Wrap(err, "unexpected EC error")
			}
			return nil
		}, &testing.PollOptions{Timeout: 60 * time.Second, Interval: 3 * time.Second}); err != nil {
			s.Fatal("EC did not become unresponsive: ", err)
		}
		s.Log("EC is unresponsive")

		// Wait for a 60-second-delay after sending the batterycutoff command, per the test requirement on SMP battery.
		s.Logf("Sleep for %s", sleepDuration)
		if err := testing.Sleep(ctx, sleepDuration); err != nil {
			return errors.Wrap(err, "failed to sleep")
		}
		return nil
	}

	// This function will try to reconnect to the DUT and check the system power state to assure DUT has booted.
	confirmBoot := func(ctx context.Context, wakeByAC bool) error {
		// Wait for a connection to the DUT.
		s.Log("Wait for SSH to DUT")
		waitConnectCtx, cancelWaitConnect := context.WithTimeout(ctx, 3*time.Minute)
		defer cancelWaitConnect()

		if err := h.WaitConnect(waitConnectCtx); err != nil {
			if wakeByAC && errors.Is(err, context.DeadlineExceeded) {
				return &reconnectErr{E: errors.New("timed out reconnecting DUT. Attempting a press on power button")}
			}
			return errors.Wrap(err, "failed to reconnect to DUT")
		}
		return nil
	}

	// Open CCD at the end of the test if it's locked. Also, disable write protections
	// that were enabled during this test, so that other tests would not be affected
	// later (i.e. setting gbb flags to change boot mode).
	var (
		hardwareWPEnabled   bool
		ecSoftwareWPEnabled bool
		apSoftwareWPEnabled bool
	)
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Minute)
	defer cancel()

	defer func(ctx context.Context, hardwareWPEnabled, apSoftwareWPEnabled, ecSoftwareWPEnabled *bool) {
		// Cr50 goes to sleep when battery is disconnected, and when DUT wakes, CCD state might be locked.
		if hasCCD, err := h.Servo.HasCCD(ctx); err != nil {
			s.Fatal("While checking if servo has a CCD connection: ", err)
		} else if hasCCD {
			if val, err := h.Servo.GetString(ctx, servo.GSCCCDLevel); err != nil {
				s.Fatal("Failed to get gsc_ccd_level: ", err)
			} else if val != servo.Open {
				s.Logf("CCD is not open, got %q. Attempting to unlock", val)
				if err := h.Servo.SetString(ctx, servo.CR50Testlab, servo.Open); err != nil {
					s.Fatal("Failed to unlock CCD: ", err)
				}
			}
		}
		if *hardwareWPEnabled {
			s.Log("Disabling hardware write protect")
			if err := h.Servo.SetFWWPState(ctx, servo.FWWPStateOff); err != nil {
				s.Fatal("Failed to disable hardware write protect: ", err)
			}
			s.Log("Rebooting DUT to ensure hardware WP disabled")
			if err := h.Servo.SetPowerState(ctx, servo.PowerStateReset); err != nil {
				s.Fatal("Faild to reset DUT: ", err)
			}
			h.CloseRPCConnection(ctx)
			if err := h.WaitConnect(ctx); err != nil {
				s.Fatal("Failed to reconnect to DUT: ", err)
			}
		}
		if *apSoftwareWPEnabled {
			s.Log("Disabling ap software write protect")
			if err := h.RequireBiosServiceClient(ctx); err != nil {
				s.Fatal("Failed to connect to the bios service on the DUT: ", err)
			}
			if _, err := h.BiosServiceClient.SetAPSoftwareWriteProtect(ctx, &pb.WPRequest{
				Enable: false,
			}); err != nil {
				s.Fatal("Failed to disable AP write protection: ", err)
			}
		}
		if *ecSoftwareWPEnabled {
			s.Log("Disabling ec software write protect")
			if err := s.DUT().Conn().CommandContext(ctx, "ectool", "flashprotect", "disable").Run(ssh.DumpLogOnError); err != nil {
				s.Fatal("Failed to disable ec write protect: ", err)
			}
		}
	}(cleanupCtx, &hardwareWPEnabled, &apSoftwareWPEnabled, &ecSoftwareWPEnabled)

	// Check ec and ap software write protect status.
	// Enable write protections before battery cutoff.
	for _, programmer := range []string{"ec", "host"} {
		wpStatus, err := s.DUT().Conn().CommandContext(ctx, "flashrom", "-p", programmer, "--wp-status").Output(ssh.DumpLogOnError)
		if err != nil {
			s.Fatal("Failed to check for ec write protection: ", err)
		}
		if reWPEnabled := regexp.MustCompile(`WP: write protect is enabled`); !reWPEnabled.Match(wpStatus) {
			s.Logf("Enabling %s software write protect", programmer)
			switch programmer {
			case "ec":
				if err := s.DUT().Conn().CommandContext(ctx, "ectool", "flashprotect", "enable").Run(ssh.DumpLogOnError); err != nil {
					s.Fatal("Failed to enable EC software write protect: ", err)
				}
				ecSoftwareWPEnabled = true
			case "host":
				bs := pb.NewBiosServiceClient(h.RPCClient.Conn)
				if _, err := bs.SetAPSoftwareWriteProtect(ctx, &pb.WPRequest{
					Enable: true,
				}); err != nil {
					s.Fatal("Failed to enable AP write protection: ", err)
				}
				apSoftwareWPEnabled = true
			}
		}
	}

	// Check and enable hardware write protect.
	hardwareWP, err := h.Servo.GetString(ctx, servo.FWWPState)
	if err != nil {
		s.Fatal("Failed to get write protect state: ", err)
	}
	if servo.FWWPStateValue(hardwareWP) != servo.FWWPStateOn {
		s.Log("Enabling hardware write protect")
		if err := h.Servo.SetFWWPState(ctx, servo.FWWPStateOn); err != nil {
			s.Fatal("Failed to enable hardware write protect: ", err)
		}
		hardwareWPEnabled = true
	}

	// Send battery cutoff and check EC is unresponsive.
	if err := sendingBatterryCutoff(ctx); err != nil {
		s.Fatal("Failed to send Batterycutoff command and wait: ", err)
	}

	// Connect charger.
	s.Log("Starting power supply")
	if err := h.SetDUTPower(ctx, true); err != nil {
		s.Fatal("Failed to attach the charger: ", err)
	}

	// Confirm a successful boot.
	if err := confirmBoot(ctx, true); err != nil {
		if _, ok := err.(*reconnectErr); ok {
			s.Log("Context error: ", err.(*reconnectErr))
			// Power button is another wake up pin to wake DUT from deep sleep.
			// If re-connecting charger fails in waking up DUT, try with a press
			// on power to fully wake DUT from G3 into S0.
			if err := wakeDUTS0(ctx, h); err != nil {
				s.Fatal("Unable to reconnect to DUT: ", err)
			}
		} else {
			s.Fatal("Failed to boot: ", err)
		}
	}
	s.Log("DUT booted succesfully")

	// If is a CHROMESLATE and a micro-servo is connected, repeat the test but wake up the DUT with power button.
	if ffIsChromeslate && hasMicroOrC2D2 {
		s.Log("Performing extra steps for CHROMESLATE")
		// Send battery cutoff and check EC is unresponsive.
		if err := sendingBatterryCutoff(ctx); err != nil {
			s.Fatal("Failed to send Batterycutoff command and wait: ", err)
		}

		// Attempt to boot DUT by pressing power button.
		s.Log("Pressing power key")
		if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.Dur(h.Config.HoldPwrButtonPowerOff)); err != nil {
			s.Fatal("Failed to press power button")
		}

		// Confirm a successful boot.
		if err := confirmBoot(ctx, false); err != nil {
			s.Fatal("Failed to boot: ", err)
		}
		s.Log("DUT booted succesfully")
	} else if ffIsChromeslate && !hasMicroOrC2D2 {
		// During this test, the EC will become unresponsive and a micro-servo will be required to press the powerkey.
		s.Log("WARNING: DUT is a chromeslate but no micro-servo is present")
	}
}

// wakeDUTS0 will check DUT's power state if replugging AC fails to fully awake DUT from battery cutoff.
// If DUT is at G3 or S5 power button will be pressed to advance it to S0.
func wakeDUTS0(ctx context.Context, h *firmware.Helper) error {
	retryCtx, cancelRetry := context.WithTimeout(ctx, 3*time.Minute)
	defer cancelRetry()

	// Check if DUT is at G3. If DUT is in G3, use power button to boot it into S0.
	testing.ContextLog(retryCtx, "Checking if power state is at G3 or S5")
	if err := h.WaitForPowerStates(retryCtx, firmware.PowerStateInterval, 1*time.Minute, "G3", "S5"); err != nil {
		return errors.Wrap(err, "unable to get power state at G3 or S5. DUT disconnected due to other reasons")
	}
	testing.ContextLogf(retryCtx, "Pressing power button for %s to wake DUT into S0 from G3 or S5", h.Config.HoldPwrButtonPowerOn)
	if err := h.Servo.KeypressWithDuration(retryCtx, servo.PowerKey, servo.Dur(h.Config.HoldPwrButtonPowerOn)); err != nil {
		return errors.Wrap(err, "failed to press power button")
	}
	testing.ContextLog(retryCtx, "Waiting for power state S0")
	if err := h.WaitForPowerStates(retryCtx, firmware.PowerStateInterval, 1*time.Minute, "S0"); err != nil {
		return errors.Wrap(err, "unable to get power state at S0")
	}
	return nil
}
