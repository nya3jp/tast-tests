// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WilcoChargeScheduling,
		Desc:         "Checks that basic charge scheduling works on wilco devices",
		Contacts:     []string{"ncrews@chromium.org", "chromeos-power@google.com"},
		Attr:         []string{"disabled", "informational"},
		SoftwareDeps: []string{"wilco"},
		// The EC seems to poll the Peak Shift settings in a ~60 second loop, so
		// it can take up to 80 seconds for policy changes to take effect.
		// See b/138612166 for a request to change the EC behavior.
		Timeout: 4 * time.Minute,
	})
}

// WilcoChargeScheduling tests Peak Shift behavior on Wilco devices.
// For a description of this feature, see
// https://www.chromium.org/administrators/policy-list-3#DevicePowerPeakShiftEnabled
// This doesn't check all the features of Peak Shift, because that would be too
// slow. Instead, this is an integration test to ensure that the kernel is able
// to communicate with the EC and get some sort of charging behavior change.
// For a complete test of *all* the Peak Shift behaviors, there needs to be a
// separate test that is not run automatically. It would only need to get run
// to qualify a new EC firmware blob.
func WilcoChargeScheduling(fullCtx context.Context, s *testing.State) {
	// If the main body of the test times out, we still want to reserve a few
	// seconds to allow for our cleanup code to run.
	deadline, ok := fullCtx.Deadline()
	if !ok {
		s.Fatal("Test context does not have a timeout")
	}
	ctx, cancel := context.WithDeadline(fullCtx, deadline.Add(-10*time.Second))
	defer cancel()

	const (
		chargeSchedulingDir = "/sys/bus/platform/devices/wilco-charge-schedule/"
		// The hwclock command uses the "dd mmm yyyy HH:MM" format, so this
		// is the corresponding format string for time.Format().
		hwclockDateFormat = "02 Jan 2006 03:04"
		// To make tests consistent, let's set the RTC's date to
		// Monday, Jan 1, 2001, at noon.
		testingTime    = "01 Jan 2001 12:00"
		testingWeekday = "monday"
	)
	peakShiftDir := filepath.Join(chargeSchedulingDir, "peak_shift")
	peakShiftEnablePath := filepath.Join(peakShiftDir, "enable")
	peakShiftBatteryThresholdPath := filepath.Join(peakShiftDir, "battery_threshold")
	peakShiftSchedulePath := filepath.Join(peakShiftDir, testingWeekday)
	policyChangeTimeout := 80 * time.Second

	// Set the EC's RTC using the "hwclock" command. This only changes the
	// external clock on the EC, it does not change the OS/system time.
	setHwClock := func(setCtx context.Context, t time.Time) {
		dateString := strings.ToUpper(t.Format(hwclockDateFormat))
		s.Logf("Setting the RTC time to %q", dateString)
		dateArg := fmt.Sprintf("--date=%s", dateString)
		// Use localtime instead of UTC and don't mess with /etc/adjtime/
		cmd := testexec.CommandContext(setCtx, "hwclock", "--set", "--noadjfile", "--rtc=/dev/rtc1", "--localtime", dateArg)
		if err := cmd.Run(testexec.DumpLogOnError); err != nil {
			s.Fatal("Failed to set time with 'hwclock' command: ", err)
		}
	}

	readFromFile := func(path string) string {
		res, err := ioutil.ReadFile(path)
		if err != nil {
			s.Fatalf("Failed to read from %s: %v", path, err)
		}
		return string(res)
	}

	writeToFile := func(path, data string) {
		if err := ioutil.WriteFile(path, []byte(data), 0000); err != nil {
			s.Fatalf("Failed to write %q to %s: %v", data, path, err)
		}
	}

	// Poll the current charging status for a while, and if it doesn't become
	// what we expect before the timeout, then fail the test.
	verifyACUsage := func(expected bool) {
		pollChargingStatus := func(pollCtx context.Context) error {
			status, err := power.GetStatus(ctx)
			if err != nil {
				s.Fatal("Failed to get power status: ", err)
				return err
			}
			if status.LinePowerConnected != expected {
				return errors.Errorf("AC presence is %v, but should be %v", status.LinePowerConnected, expected)
			}
			return nil
		}
		opts := testing.PollOptions{Timeout: policyChangeTimeout, Interval: time.Second}
		if err := testing.Poll(ctx, pollChargingStatus, &opts); err != nil {
			s.Fatal("Charging status never became correct: ", err)
		}
	}

	// Ensure the DUT is back in it's original state after the test.
	defer func() {
		setHwClock(fullCtx, time.Now())
	}()
	defer writeToFile(peakShiftEnablePath, readFromFile(peakShiftEnablePath))
	defer writeToFile(peakShiftBatteryThresholdPath, readFromFile(peakShiftBatteryThresholdPath))
	defer writeToFile(peakShiftSchedulePath, readFromFile(peakShiftSchedulePath))

	// Set the RTC time to a consistent dummy time.
	t, err := time.Parse(hwclockDateFormat, testingTime)
	if err != nil {
		s.Fatal("Failed to parse the testing HwClock time: ", err)
	}
	setHwClock(ctx, t)

	s.Log("Enabling peakshift and setting a low battery threshold")
	s.Log("Setting schedule to use full peakshift, waiting for the DUT to run on just battery")
	writeToFile(peakShiftEnablePath, "1")
	writeToFile(peakShiftBatteryThresholdPath, "15")
	writeToFile(peakShiftSchedulePath, "01:00 22:00 23:00")
	verifyACUsage(false)

	s.Log("Setting schedule to disable Peak Shift, waiting for the DUT to run on AC with battery charging")
	writeToFile(peakShiftEnablePath, "1")
	writeToFile(peakShiftBatteryThresholdPath, "15")
	writeToFile(peakShiftSchedulePath, "01:00 02:00 03:00")
	verifyACUsage(true)

	// Toggle back one more time to ensure that the first time we enabled Peak
	// Shift it wasn't affected by some external shift environment.
	s.Log("Setting schedule to use full peakshift, waiting for the DUT to run on just battery")
	writeToFile(peakShiftEnablePath, "1")
	writeToFile(peakShiftBatteryThresholdPath, "15")
	writeToFile(peakShiftSchedulePath, "01:00 22:00 23:00")
	verifyACUsage(false)
}
