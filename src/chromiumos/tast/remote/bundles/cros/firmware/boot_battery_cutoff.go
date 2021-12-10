// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware/fixture"
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

func init() {
	testing.AddTest(&testing.Test{
		Func:         BootBatteryCutoff,
		Desc:         "Verify if system can boot after battery cutoff",
		Contacts:     []string{"cienet-firmware@cienet.corp-partner.google.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable"},
		Fixture:      fixture.NormalMode,
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.SkipOnPlatform("fizz", "jecht"), hwdep.SkipOnFormFactor(hwdep.Chromebox, hwdep.Chromebase)),
		Params: []testing.Param{{
			Name:              "chromeslate",
			ExtraHardwareDeps: hwdep.D(hwdep.FormFactor(hwdep.Chromeslate)),
			Val:               true,
		}, {
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnFormFactor(hwdep.Chromeslate)),
			Val:               false,
		}},
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
		}, &testing.PollOptions{Timeout: 15 * time.Second, Interval: 3 * time.Second}); err != nil {
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
	confirmBoot := func(ctx context.Context) error {
		// Wait for a connection to the DUT.
		s.Log("Wait for SSH to DUT")
		waitConnectCtx, cancelWaitConnect := context.WithTimeout(ctx, 2*time.Minute)
		defer cancelWaitConnect()

		if err := h.WaitConnect(waitConnectCtx); err != nil {
			return errors.Wrap(err, "failed to reconnect to DUT")
		}
		return nil
	}

	// This function will enable AP software write protect.
	enableAPWriteProtect := func(ctx context.Context) error {
		// Check AP firmware WP range.
		if err := s.DUT().Conn().CommandContext(ctx, "flashrom", "-p", "host", "-r", "/tmp/bios.bin").Run(ssh.DumpLogOnError); err != nil {
			return errors.Wrap(err, "failed to read the bios file")
		}

		out, err := s.DUT().Conn().CommandContext(ctx, "fmap_decode", "/tmp/bios.bin").Output(ssh.DumpLogOnError)
		if err != nil {
			return errors.Wrap(err, "failed to decode the bios file")
		}

		// Parse the output to get the areaOffset and areaSize values for write protection.
		stringv := strings.Split(string(out), "\n")
		var areaOffset string
		var areaSize string
		for _, line := range stringv {
			if strings.Contains(line, "WP_RO") {
				values := strings.Split(line, "\"")
				areaOffset = values[1]
				areaSize = values[3]
				break
			}
		}

		// Declare the starting and ending range to run in the flashrom command for write protection.
		command := fmt.Sprintf("%v,%v", areaOffset, areaSize)
		if err = s.DUT().Conn().CommandContext(ctx, "flashrom", "-p", "host", "--wp-enable", "--wp-range", command).Run(ssh.DumpLogOnError); err != nil {
			return errors.Wrap(err, "failed to enable AP software write protect")
		}
		return nil
	}

	// Enable software write protect.
	s.Log("Enabling EC software write protect")
	if err := s.DUT().Conn().CommandContext(ctx, "ectool", "flashprotect", "enable").Run(ssh.DumpLogOnError); err != nil {
		if out, err := s.DUT().Conn().CommandContext(ctx, "flashrom", "-p", "ec", "--wp-status").Output(ssh.DumpLogOnError); err != nil {
			s.Error("Failed to run command flashrom to collect the write protection status: ", err)
		} else {
			s.Log("Flashrom Output: ", string(out))
		}
		s.Fatal("Failed to enable EC software write protect: ", err)
	}

	s.Log("Enabling AP software write protect")
	if err := enableAPWriteProtect(ctx); err != nil {
		s.Fatal("While attempting to enable AP write protection: ", err)
	}

	// Enable hardware write protect.
	s.Log("Enabling hardware write protect")
	if err := h.Servo.SetFWWPState(ctx, servo.FWWPStateOn); err != nil {
		s.Fatal("Failed to enable hardware write protect: ", err)
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
	if err := confirmBoot(ctx); err != nil {
		s.Fatal("Failed to boot: ", err)
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
		if err := confirmBoot(ctx); err != nil {
			s.Fatal("Failed to boot: ", err)
		}
		s.Log("DUT booted succesfully")
	} else if ffIsChromeslate && !hasMicroOrC2D2 {
		// During this test, the EC will become unresponsive and a micro-servo will be required to press the powerkey.
		s.Log("WARNING: DUT is a chromeslate but no micro-servo is present")
	}

}
