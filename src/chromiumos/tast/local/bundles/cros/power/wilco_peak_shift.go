// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/rtc"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: WilcoPeakShift,
		Desc: "Checks that basic Peak Shift works on Wilco devices",
		Contacts: []string{
			"ncrews@chromium.org",       // Test author and EC kernel driver author.
			"chromeos-wilco@google.com", // Possesses some more domain-specific knowledge.
			"chromeos-kernel@google.com",
			"chromeos-power@google.com",
		},
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
//  - Use AC and charge the battery (max AC usage)
//  - Use AC, but don't charge the battery (medium AC usage)
//  - Don't use AC, and run off just the battery. (no AC usage)
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
// charging/no-charging. This serves mostly as an integration test, to check
// that we can communicate with the EC.
//
// TODO(b/138940522): Add a more thorough, slower, manual test for
// all aspects of the policy.
func WilcoPeakShift(ctx context.Context, s *testing.State) {
	// If the main body of the test times out, we still want to reserve a few
	// seconds to allow for our cleanup code to run.
	cleanupCtx := ctx
	mainCtx, cancel := ctxutil.Shorten(cleanupCtx, 5*time.Second)
	defer cancel()

	const (
		// Location of sysfs files that control Peak Shift
		peakShiftDir = "/sys/bus/platform/devices/wilco-charge-schedule/peak_shift/"
		// See the note on the test timeout in test declaration.
		policyChangeTimeout = 80 * time.Second
		// There is an upstart job that continually keeps the EC RTC in sync with
		// local time. We need to disable it during the test.
		upstartJobName = "wilco_sync_ec_rtc"
	)
	wilcoECRTC := rtc.RTC{DevName: "rtc1", LocalTime: true, NoAdjfile: true}
	// To be consistent, let's set the RTC to Jan 1, 2001, at noon, local time.
	testingTime := time.Date(2001, time.January, 1, 12, 0, 0, 0, time.Now().Location())
	// Should be "monday"
	testingWeekdayString := strings.ToLower(testingTime.Weekday().String())
	peakShiftSchedulePath := filepath.Join(peakShiftDir, testingWeekdayString)
	peakShiftEnablePath := filepath.Join(peakShiftDir, "enable")
	peakShiftBatteryThresholdPath := filepath.Join(peakShiftDir, "battery_threshold")

	writeECRTC := func(ctx context.Context, t time.Time) {
		if err := wilcoECRTC.Write(ctx, t); err != nil {
			s.Fatal("Failed to write EC RTC: ", err)
		}
	}

	readFileStrict := func(path string) string {
		res, err := ioutil.ReadFile(path)
		if err != nil {
			s.Fatalf("Failed to read from %s: %v", path, err)
		}
		return string(res)
	}

	writeFileStrict := func(path, data string) {
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
		pollPowerStatus := func(pollCtx context.Context) error {
			status, err := power.GetStatus(pollCtx)
			if err != nil {
				s.Fatal("Failed to get power status: ", err)
			}
			// In order for Peak Shift to disconnect line power the battery must be
			// appreciably above the 15% battery threshold.
			if status.BatteryPercent < 20 {
				s.Fatalf("Battery level is %v, must be at least %v to begin test", status.BatteryPercent, 20)
			}
			if status.LinePowerConnected != expected {
				return errors.Errorf("AC presence is %v, but should be %v", status.LinePowerConnected, expected)
			}
			return nil
		}
		opts := testing.PollOptions{Timeout: policyChangeTimeout}
		if err := testing.Poll(mainCtx, pollPowerStatus, &opts); err != nil {
			s.Fatal("AC presence never became correct: ", err)
		}
	}

	// Stop the upstart job that keeps the EC RTC in sync with local time.
	if err := upstart.StopJob(mainCtx, upstartJobName); err != nil {
		s.Fatal("Failed to stop sync RTC upstart job: ", err)
	}
	defer func() {
		if err := upstart.EnsureJobRunning(cleanupCtx, upstartJobName); err != nil {
			s.Fatal("Failed to restart sync RTC upstart job: ", err)
		}
	}()

	// Set the RTC time to a consistent dummy time.
	writeECRTC(mainCtx, testingTime)
	defer func() {
		writeECRTC(cleanupCtx, time.Now())
	}()

	// Ensure (as best we can) that the DUT is back in it's original state after the test.
	defer writeFileStrict(peakShiftEnablePath, readFileStrict(peakShiftEnablePath))
	defer writeFileStrict(peakShiftBatteryThresholdPath, readFileStrict(peakShiftBatteryThresholdPath))
	defer writeFileStrict(peakShiftSchedulePath, readFileStrict(peakShiftSchedulePath))

	s.Log("Enabling Peak Shift and setting lowest possible battery threshold")
	writeFileStrict(peakShiftEnablePath, "1")
	writeFileStrict(peakShiftBatteryThresholdPath, "15")

	s.Log("Setting schedule to disable Peak Shift, waiting for the DUT to use AC")
	writeFileStrict(peakShiftSchedulePath, "01:00 02:00 03:00")
	verifyACUsage(true)

	s.Log("Setting schedule to use full Peak Shift, waiting for the DUT to not use AC")
	writeFileStrict(peakShiftSchedulePath, "01:00 22:00 23:00")
	verifyACUsage(false)

	s.Log("Setting schedule to disable Peak Shift, waiting for the DUT to use AC")
	writeFileStrict(peakShiftSchedulePath, "01:00 02:00 03:00")
	verifyACUsage(true)
}
