// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/wilco"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     WilcoAdvancedCharging,
		Desc:     "Checks that basic Advanced Charging works on Wilco devices",
		Contacts: []string{"ncrews@chromium.org", "chromeos-power@google.com"},
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
	const (
		// Location of sysfs files that control Advanced Charging
		advancedChargingDir = "/sys/bus/platform/devices/wilco-charge-schedule/advanced_charging/"
		policyChangeTimeout = 5 * time.Second
	)
	// To be consistent, let's set the RTC to Jan 1, 2001, at noon, local time.
	testingTime := time.Date(2001, time.January, 1, 12, 0, 0, 0, time.Now().Location())
	// Should be "monday"
	testingWeekdayString := strings.ToLower(testingTime.Weekday().String())
	advancedChargingEnablePath := filepath.Join(advancedChargingDir, "enable")
	advancedChargingSchedulePath := filepath.Join(advancedChargingDir, testingWeekdayString)

	// To be able to differentiate between charging modes we need to be able to
	// charge, with the battery level above 90%.
	verifyTestableState := func(status *power.Status) error {
		if !status.LinePowerConnected || status.BatteryPercent < 90 || status.BatteryStatus == "Full" {
			return errors.Errorf("not in a testable state: AC=%v with battery=%v%% with status %q; expected AC=true with battery>90%% with status!=\"Full\"", status.LinePowerConnected, status.BatteryPercent, status.BatteryStatus)
		}
		return nil
	}

	verifyCharging := func(expected bool) {
		pollPowerStatus := func(pollCtx context.Context) error {
			status := wilco.GetPowerStatus(ctx, s)
			if err := verifyTestableState(status); err != nil {
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

	// Ensure the DUT is back in it's original state after the test.
	defer func() {
		wilco.WriteECRTC(ctx, s, time.Now())
	}()
	defer wilco.WriteFileStrict(s, advancedChargingEnablePath, wilco.ReadFileStrict(s, advancedChargingEnablePath))
	defer wilco.WriteFileStrict(s, advancedChargingSchedulePath, wilco.ReadFileStrict(s, advancedChargingSchedulePath))

	// Stop the upstart job that keeps the EC RTC in sync with local time.
	wilco.StopSyncRTCJob(ctx, s)
	defer func() {
		wilco.StartSyncRTCJob(ctx, s)
	}()

	// Set the RTC time to a consistent dummy time.
	wilco.WriteECRTC(ctx, s, testingTime)

	wilco.WriteFileStrict(s, advancedChargingEnablePath, "0")
	// Set the schedule so we are outside of the working period.
	wilco.WriteFileStrict(s, advancedChargingSchedulePath, "01:00 02:00")
	verifyCharging(true)

	wilco.WriteFileStrict(s, advancedChargingEnablePath, "1")
	// Set the schedule so we are outside of the working period.
	wilco.WriteFileStrict(s, advancedChargingSchedulePath, "01:00 02:00")
	verifyCharging(false)

	wilco.WriteFileStrict(s, advancedChargingEnablePath, "1")
	// Set the schedule so we are inside of the working period.
	wilco.WriteFileStrict(s, advancedChargingSchedulePath, "01:00 23:00")
	verifyCharging(true)
}
