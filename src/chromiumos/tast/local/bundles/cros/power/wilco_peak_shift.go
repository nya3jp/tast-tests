// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WilcoPeakShift,
		Desc:         "Checks that basic Peak Shift works on Wilco devices",
		Contacts:     []string{"ncrews@chromium.org", "chromeos-power@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"wilco"},
		// The EC seems to poll the Peak Shift settings in a ~60 second loop, so
		// it can take up to 80 seconds for policy changes to take effect.
		// See b/138612166 for a request to change the EC behavior.
		Timeout: 4 * time.Minute,
	})
}

// WilcoPeakShift performs basic tests of the Peak Shift behavior on Wilco devices.
// The Peak Shift policy on Wilco devices uses the EC to schedule when the
// DUT uses AC power. Specifically, by using the policy it is possible to
// schedule 3 different modes:
// -Use AC and charge the battery (max AC usage)
// -Use AC, but don't charge the battery (medium AC usage)
// -Don't use AC, and run off just the battery. (no AC usage)
// For more information, see
// https://www.chromium.org/administrators/policy-list-3#DevicePowerPeakShiftEnabled
//
// In order to test all aspects of the policy, it must be possible for the
// EC to charge the battery. That would require the battery level to be
// less than 100%, but since in the lab all devices are always plugged into
// AC, the batteries are kept at 100%. In order to test the charging
// behavior this test would need to drain the batteries somewhat, which
// could take ~10 minutes. Therefore, this test only verifies that the
// device can toggle AC usage on and off, and doesn't test for battery
// charging/no-charging.
//
// TODO(b/138940522): Add a more thorough, slower, manual test for
// all aspects of the policy.
func WilcoPeakShift(fullCtx context.Context, s *testing.State) {
	// If the main body of the test times out, we still want to reserve a few
	// seconds to allow for our cleanup code to run.
	ctx, cancel := ctxutil.Shorten(fullCtx, 10*time.Second)
	defer cancel()

	const (
		chargeSchedulingDir = "/sys/bus/platform/devices/wilco-charge-schedule/"
		// The hwclock command uses the "dd mmm yyyy HH:MM" format, so this
		// is the corresponding format string for time.Format().
		hwclockDateFormat = "02 Jan 2006 15:04"
		// To make tests consistent, let's set the RTC's date to
		// Monday, Jan 1, 2001, at noon.
		testingTime    = "01 Jan 2001 12:00"
		testingWeekday = "monday"
	)
	peakShiftDir := filepath.Join(chargeSchedulingDir, "peak_shift")
	peakShiftEnablePath := filepath.Join(peakShiftDir, "enable")
	peakShiftBatteryThresholdPath := filepath.Join(peakShiftDir, "battery_threshold")
	peakShiftSchedulePath := filepath.Join(peakShiftDir, testingWeekday)
	// See the note on the test timeout in test declaration.
	policyChangeTimeout := 80 * time.Second

	// Set the EC's RTC using the "hwclock" command. This only changes the
	// external clock on the EC, it does not change the OS/system time.
	setHWClock := func(setCtx context.Context, t time.Time) {
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
		if _, err := os.Stat(path); os.IsNotExist(err) {
			s.Fatal("File does not exist: ", err)
		}
		if err := ioutil.WriteFile(path, []byte(data), 0644); err != nil {
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
		opts := testing.PollOptions{Timeout: policyChangeTimeout}
		if err := testing.Poll(ctx, pollChargingStatus, &opts); err != nil {
			s.Fatal("Charging status never became correct: ", err)
		}
	}

	// Ensure the DUT is back in it's original state after the test.
	defer func() {
		setHWClock(fullCtx, time.Now())
	}()
	defer writeToFile(peakShiftEnablePath, readFromFile(peakShiftEnablePath))
	defer writeToFile(peakShiftBatteryThresholdPath, readFromFile(peakShiftBatteryThresholdPath))
	defer writeToFile(peakShiftSchedulePath, readFromFile(peakShiftSchedulePath))

	// Set the RTC time to a consistent dummy time.
	t, err := time.Parse(hwclockDateFormat, testingTime)
	if err != nil {
		s.Fatal("Failed to parse the testing HwClock time: ", err)
	}
	setHWClock(ctx, t)

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
	// Shift it wasn't affected by some external part of the environment.
	s.Log("Setting schedule to use full peakshift, waiting for the DUT to run on just battery")
	writeToFile(peakShiftEnablePath, "1")
	writeToFile(peakShiftBatteryThresholdPath, "15")
	writeToFile(peakShiftSchedulePath, "01:00 22:00 23:00")
	verifyACUsage(false)
}
