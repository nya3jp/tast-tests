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
	pow "chromiumos/tast/local/power"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChargeScheduling,
		Desc:         "Checks that charge scheduling works on wilco devices",
		Contacts:     []string{"ncrews@chromium.org", "chromeos-power@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"wilco"},
		// The EC takes ~60 seconds to respond when we change the charge
		// scheduling settings. See b/138612166 for a request to change the EC behavior
		Timeout: 10 * time.Minute,
	})
}

type chargingStatus int

const (
	// Not using AC power, running off just the batteries
	usingBattery chargingStatus = iota
	// Running off AC power, but not charging the batteries.
	usingAC
	// Running off AC power and charging the batteries.
	usingACAndCharging
)

func (cs chargingStatus) String() string {
	return [...]string{"usingBattery", "usingAC", "usingACAndCharging"}[cs]
}

// ChargeScheduling tests Peak Shift behavior. For a description of this feature, see
// https://www.chromium.org/administrators/policy-list-3#DevicePowerPeakShiftEnabled
func ChargeScheduling(fullCtx context.Context, s *testing.State) {
	deadline, ok := fullCtx.Deadline()
	if !ok {
		s.Fatal("Test context does not have a timeout")
	}
	ctx, cancel := context.WithDeadline(fullCtx, deadline.Add(-10*time.Second))
	defer cancel()
	cleanupCtx, cancel := context.WithDeadline(fullCtx, deadline)
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
		// To verify that charging can occur, we need the batteries low enough
		// that they can charge. In the lab, the DUTs are kept plugged in so their
		// batteries are always full. From testing, by maxing out all the CPUs with
		// pow.FullyLoadCpus(), the batteries drain at about .6 percent a minute. By
		// pulling up https://webglsamples.org/aquarium/aquarium.html at the same,
		// I could get this up to .75 percent a minute.
		maxTestingBatteryPercentage = 97
	)
	psDir := filepath.Join(chargeSchedulingDir, "peak_shift")
	psEnablePath := filepath.Join(psDir, "enable")
	psBatteryThresholdPath := filepath.Join(psDir, "battery_threshold")
	psSchedulePath := filepath.Join(psDir, testingWeekday)

	// Set the EC's RTC to a specific time using the "hwclock" command.
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

	getPowerStatus := func() *pow.Status {
		status, err := pow.GetStatus(ctx)
		if err != nil {
			s.Fatal("Failed to get power status: ", err)
		}
		return status
	}

	// Read the battery and AC barrel plug state from sysfs to determine the
	// current charging behavior
	getchargingStatus := func() chargingStatus {
		status := getPowerStatus()
		if !status.LinePowerConnected {
			return usingBattery
		}
		if status.BatteryStatus == "Charging" {
			return usingACAndCharging
		}
		return usingAC
	}

	// Poll the current charging status for a while, and if it doesn't become
	// what we expect before the timeout, then fail the test. We have to poll
	// because the EC takes 15-60 seconds to respond when we change the charge
	// scheduling settings.
	verifychargingStatus := func(expected chargingStatus, timeout time.Duration) {
		pollchargingStatus := func(ctx context.Context) error {
			if cs := getchargingStatus(); cs != expected {
				return errors.Errorf("The current charging state is %v, but should be %v", cs, expected)
			}
			return nil
		}
		opts := testing.PollOptions{Timeout: timeout, Interval: time.Second}
		if err := testing.Poll(ctx, pollchargingStatus, &opts); err != nil {
			s.Fatal("Charging status never became correct: ", err)
		}
	}

	// Wait until either the battery level drops below the given percentage, or
	// the given timeout expires. To speed up the process, max out all the CPUs
	drainBatteryBelow := func(percent float64, timeout time.Duration) {
		pollBatteryLevel := func(ctx context.Context) error {
			if status := getPowerStatus(); status.BatteryPercent > percent {
				return errors.Errorf("Battery percentage should be below %v, but is %v", percent, status.BatteryPercent)
			}
			return nil
		}
		opts := testing.PollOptions{Timeout: timeout, Interval: time.Second}
		cancel := pow.FullyLoadCpus(ctx, timeout)
		defer cancel()
		if err := testing.Poll(ctx, pollBatteryLevel, &opts); err != nil {
			s.Fatal("Battery level never became low enough: ", err)
		}
	}

	// Ensure the DUT is back in it's original state after the test.
	defer func() {
		setHwClock(cleanupCtx, time.Now())
	}()
	defer writeToFile(psEnablePath, readFromFile(psEnablePath))
	defer writeToFile(psBatteryThresholdPath, readFromFile(psBatteryThresholdPath))
	defer writeToFile(psSchedulePath, readFromFile(psSchedulePath))

	// Set the RTC time to a dummy time
	t, err := time.Parse(hwclockDateFormat, testingTime)
	if err != nil {
		s.Fatal("Failed to parse the testing HwClock time: ", err)
	}
	setHwClock(ctx, t)

	s.Log("Enabling peakshift and setting a low battery threshold")
	writeToFile(psEnablePath, "1")
	writeToFile(psBatteryThresholdPath, "15")

	s.Log("Setting schedule to use full peakshift, checking if the DUT will run on just battery")
	writeToFile(psSchedulePath, "01:00 22:00 23:00")
	verifychargingStatus(usingBattery, 90*time.Second)

	s.Logf("Draining batteries below %v percent so we can verify charging", maxTestingBatteryPercentage)
	drainBatteryBelow(maxTestingBatteryPercentage, 3*time.Minute)

	s.Log("Setting schedule to use semi peakshift, checking if the DUT will run on just AC with no battery charging")
	writeToFile(psSchedulePath, "01:00 02:00 23:00")
	verifychargingStatus(usingAC, 90*time.Second)

	s.Log("Setting schedule to diable Peak Shift, checking if the DUT will run on AC with battery charging")
	writeToFile(psSchedulePath, "01:00 02:00 03:00")
	verifychargingStatus(usingACAndCharging, 90*time.Second)

	s.Log("Setting schedule to use Peak Shift, but turning off enable, checking if the DUT will use AC and charge")
	writeToFile(psSchedulePath, "01:00 22:00 23:00")
	writeToFile(psEnablePath, "0")
	verifychargingStatus(usingACAndCharging, 90*time.Second)

	s.Log("Re-enabling Peak Shift, but setting a high battery threshold, checking if the DUT will use AC and charge")
	writeToFile(psEnablePath, "1")
	writeToFile(psBatteryThresholdPath, "100")
	verifychargingStatus(usingACAndCharging, 90*time.Second)
}
