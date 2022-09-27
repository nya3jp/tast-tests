// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package typec

import (
	"context"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/typec/fixture"
	"chromiumos/tast/remote/bundles/cros/typec/typecutils"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Basic,
		Desc:         "Checks basic typec kernel driver functionality",
		Contacts:     []string{"pmalani@chromium.org", "chromeos-power@google.com"},
		Attr:         []string{"group:mainline", "group:typec", "informational"},
		HardwareDeps: hwdep.D(hwdep.ECFeatureTypecCmd(), hwdep.SkipOnModel("fievel", "habokay", "tiger"), hwdep.ChromeEC()),
		Vars:         []string{"servo"},
		Fixture:      "typeCServo",
	})
}

// Basic does the following:
// - Simulate a servo disconnect.
// - Reconfigure the servo as DP device supporting pin assignment C.
// - Reconnect the servo.
// - Verify that the kernel recognizes the servo partner and can parse its DP VDO data.
//
// It then repeats the process with the servo configured as a pin assignment D DP device.
//
// Since it's not possible to verify that the DUT detected a disconnect (since the DUT loses its
// network connection during servo disconnect), we check the DUT uptime before and after the
// test. If the end time is greater than the start time, we can infer that the partner
// detected was due to a hotplug and not at reboot (since the partner PD data gets parsed only once
// on each connect).
func Basic(ctx context.Context, s *testing.State) {
	d := s.DUT()
	startTime, err := getUpTime(ctx, d)
	if err != nil {
		s.Fatal("Failed to get DUT uptime: ", err)
	} else if startTime == 0 {
		s.Fatal("DUT didn't return a valid uptime")
	}

	svo := s.FixtValue().(*fixture.Value).Servo()

	s.Log("Checking DP pin C")
	if err := runDPTest(ctx, svo, d, s, "c"); err != nil {
		s.Fatal("DP pin C check failed: ", err)
	}

	s.Log("Checking DP pin D")
	if err := runDPTest(ctx, svo, d, s, "d"); err != nil {
		s.Fatal("DP pin D check failed: ", err)
	}

	endTime, err := getUpTime(ctx, d)
	if err != nil {
		s.Fatal("Failed to get DUT uptime: ", err)
	}

	// Check if we might have undergone a reboot.
	if endTime < startTime {
		s.Fatalf("End uptime (%d) lower than start uptime (%d); suggests unexpected reboot", endTime, startTime)
	} else if endTime == 0 {
		s.Fatal("DUT didn't return a valid uptime")
	}
}

// runDPTest performs the DP alternate mode detection test for a specified pin assignment.
// Returns nil on success, otherwise the error message.
func runDPTest(ctx context.Context, svo *servo.Servo, d *dut.DUT, s *testing.State, pinAssign string) error {

	s.Log("Simulating servo disconnect")
	if err := typecutils.CcOffAndWait(ctx, svo); err != nil {
		return errors.Wrap(err, "failed CC off and wait")
	}

	if err := d.Disconnect(ctx); err != nil {
		return errors.Wrap(err, "failed to close the current DUT ssh connection")
	}

	s.Log("Configuring Servo to enable DP")
	if err := setServoDPMode(ctx, svo, pinAssign); err != nil {
		return errors.Wrap(err, "failed to configure servo for DP")
	}

	s.Log("Simulating servo reconnect")
	if err := svo.SetCC(ctx, servo.On); err != nil {
		return errors.Wrap(err, "failed to switch on CC")
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return d.Connect(ctx)
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to connect to DUT")
	}

	// Wait for PD negotiation to stabilize.
	if err := testing.Sleep(ctx, 2500*time.Millisecond); err != nil {
		return errors.Wrap(err, "failed to sleep for PD negotiation")
	}

	// Check that the partner DP alternate mode is found.
	if err := typecutils.CheckForDPAltMode(ctx, d, s, pinAssign); err != nil {
		return errors.Wrap(err, "failed to find the expected partner")
	}

	return nil
}

// getUpTime is a utility function that returns the seconds value of "/proc/uptime"
// from the DUT, else 0 along with an error message in case of an error.
func getUpTime(ctx context.Context, d *dut.DUT) (int, error) {
	out, err := linuxssh.ReadFile(ctx, d.Conn(), "/proc/uptime")
	if err != nil {
		return 0, errors.Wrap(err, "could not run cat /proc/uptime on the DUT")
	}

	// The first float constitutes time since power on.
	re := regexp.MustCompile(`\d+\.\d+`)
	timeStr := re.FindString(string(out))
	if timeStr != "" {
		f, err := strconv.ParseFloat(timeStr, 64)
		if err != nil {
			return 0, errors.Wrap(err, "coudn't parse uptime float value")
		}
		return int(f), nil
	}

	return 0, errors.New("couldn't find a valid uptime")
}

// setServoDPMode runs some servo console commands to configure the servo to advertise
// DP alternate mode support with the selected pin assignment setting.
func setServoDPMode(ctx context.Context, svo *servo.Servo, pinAssign string) error {
	if err := svo.RunUSBCDPConfigCommand(ctx, "disable"); err != nil {
		return errors.Wrap(err, "failed to disable DP support")
	}

	if err := svo.RunUSBCDPConfigCommand(ctx, "pins", pinAssign); err != nil {
		return errors.Wrap(err, "failed to set DP pin assignment")
	}

	if err := svo.RunUSBCDPConfigCommand(ctx, "mf", "0"); err != nil {
		return errors.Wrap(err, "failed to set DP multi-function")
	}

	if err := svo.RunUSBCDPConfigCommand(ctx, "enable"); err != nil {
		return errors.Wrap(err, "failed to enable DP support")
	}

	return nil
}
