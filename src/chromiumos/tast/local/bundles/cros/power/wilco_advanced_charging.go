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
		Func: WilcoAdvancedCharging,
		Desc: "Checks that basic Advanced Charging works on Wilco devices",
		Contacts: []string{
			"ncrews@chromium.org",       // Test author and EC kernel driver author.
			"chromeos-wilco@google.com", // Possesses some more domain-specific knowledge.
			"chromeos-kernel@google.com",
			"chromeos-power@google.com",
		},
		// Because this test requires the battery to be in a certain state, this
		// test is marked "disabled" so that it does not run in the CQ.
		Attr:         []string{"disabled", "informational"},
		SoftwareDeps: []string{"wilco"},
		Timeout:      2 * time.Minute,
	})
}

// WilcoAdvancedCharging tests the Advanced Charging policy behavior on Wilco
// devices. The Advanced Charging policy on Wilco devices uses the EC to
// schedule different battery charging policies. When the policy is disabled,
// the device acts as normal. When the policy is enabled, then
//  - If we are outside working hours, then above 90% the battery will not charge.
//  - If we are within working hours, then the battery will charge to 100%.
// The policy also affects the charging rate, but it is easier just to test for
// charging/no-charging. This serves mostly as an integration test, to check
// that we can communicate with the EC. See the following link for more info:
// https://www.chromium.org/administrators/policy-list-3#DeviceAdvancedBatteryChargeModeEnabled
func WilcoAdvancedCharging(ctx context.Context, s *testing.State) {
	// If the main body of the test times out, we still want to reserve a few
	// seconds to allow for our cleanup code to run.
	cleanupCtx := ctx
	mainCtx, cancel := ctxutil.Shorten(cleanupCtx, 5*time.Second)
	defer cancel()

	const (
		// Location of sysfs files that control Advanced Charging
		advancedChargingDir = "/sys/bus/platform/devices/wilco-charge-schedule/advanced_charging/"
		// The EC takes a few seconds to adjust charging behavior after a policy change.
		policyChangeTimeout = 5 * time.Second
		// There is an upstart job that continually keeps the EC RTC in sync with
		// local time. We need to disable it during the test.
		upstartJobName = "wilco_sync_ec_rtc"
	)
	wilcoECRTC := rtc.RTC{DevName: "rtc1", LocalTime: true, NoAdjfile: true}
	// To be consistent, let's set the RTC to Jan 1, 2001, at noon, local time.
	testingTime := time.Date(2001, time.January, 1, 12, 0, 0, 0, time.Now().Location())
	// Should be "monday"
	testingWeekdayString := strings.ToLower(testingTime.Weekday().String())
	advancedChargingSchedulePath := filepath.Join(advancedChargingDir, testingWeekdayString)
	advancedChargingEnablePath := filepath.Join(advancedChargingDir, "enable")

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

	verifyCharging := func(expected bool) {
		pollPowerStatus := func(pollCtx context.Context) error {
			status, err := power.GetStatus(pollCtx)
			if err != nil {
				s.Fatal("Failed to get power status: ", err)
			}
			// To be able to differentiate between charging modes we need to be able to
			// charge, with the battery level above 90%.
			if !status.LinePowerConnected || status.BatteryPercent < 90 || status.BatteryStatus == "Full" {
				err := errors.Errorf("not in a testable state: AC=%v with battery=%v%% with status %q; expected AC=true with battery>90%% with status!=\"Full\"", status.LinePowerConnected, status.BatteryPercent, status.BatteryStatus)
				return testing.PollBreak(err)
			}
			charging := status.BatteryCurrent > .01
			if charging != expected {
				return errors.Errorf("charging=%v, but should be %v", charging, expected)
			}
			return nil
		}
		opts := testing.PollOptions{Timeout: policyChangeTimeout}
		if err := testing.Poll(ctx, pollPowerStatus, &opts); err != nil {
			s.Fatal("Power status is incorrect: ", err)
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

	// Ensure the DUT is back in it's original state after the test.
	defer writeFileStrict(advancedChargingEnablePath, readFileStrict(advancedChargingEnablePath))
	defer writeFileStrict(advancedChargingSchedulePath, readFileStrict(advancedChargingSchedulePath))

	s.Log("Disabling the policy and setting the schedule so we are outside of the working period, waiting for DUT to charge")
	writeFileStrict(advancedChargingEnablePath, "0")
	writeFileStrict(advancedChargingSchedulePath, "01:00 02:00")
	verifyCharging(true)

	s.Log("Enabling the policy and setting the schedule so we are outside of the working period, waiting for DUT to not charge")
	writeFileStrict(advancedChargingEnablePath, "1")
	writeFileStrict(advancedChargingSchedulePath, "01:00 02:00")
	verifyCharging(false)

	s.Log("Enabling the policy and setting the schedule so we are inside of the working period, waiting for DUT to charge")
	writeFileStrict(advancedChargingEnablePath, "1")
	writeFileStrict(advancedChargingSchedulePath, "01:00 23:00")
	verifyCharging(true)
}
