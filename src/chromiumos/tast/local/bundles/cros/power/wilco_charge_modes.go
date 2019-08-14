// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: WilcoChargeModes,
		Desc: "Checks that the basic Charge Mode works on Wilco devices",
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
		Timeout:      30 * time.Second,
	})
}

// WilcoChargeModes tests basic control of the various charge modes that the
// Wilco EC provides. Specifically, it checks that we can control whether or
// not charging happens by adjusting the Charge Stop Theshold while in Custom
// mode. This test is intended as an integration test and fails to check for
// the various other aspects of the Charge Mode policy.
func WilcoChargeModes(ctx context.Context, s *testing.State) {
	const (
		// Location of sysfs files that control Charge Mode
		chargerDir = "/sys/class/power_supply/wilco-charger/"
		// The EC takes a few seconds to adjust charging behavior after a policy change.
		policyChangeTimeout = 5 * time.Second
	)
	chargeModePath := filepath.Join(chargerDir, "charge_type")
	chargeStartThresholdPath := filepath.Join(chargerDir, "charge_control_start_threshold")
	chargeEndThresholdPath := filepath.Join(chargerDir, "charge_control_end_threshold")

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
			// charge, with the battery level above 55%.
			if !status.LinePowerConnected || status.BatteryPercent < 55 || status.BatteryStatus == "Full" {
				err := errors.Errorf("not in a testable state: AC=%v with battery=%v%% with status %q; expected AC=true with battery>55%% with status!=\"Full\"", status.LinePowerConnected, status.BatteryPercent, status.BatteryStatus)
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
			s.Fatal("Charge Mode is not correct: ", err)
		}
	}

	// Ensure (as best we can) that the DUT is back in it's original state after the test.
	defer writeFileStrict(chargeModePath, readFileStrict(chargeModePath))
	defer writeFileStrict(chargeStartThresholdPath, readFileStrict(chargeStartThresholdPath))
	defer writeFileStrict(chargeEndThresholdPath, readFileStrict(chargeEndThresholdPath))

	// Set the start threshold as low as possible so that we are always above it,
	// and thus the end threshold is the determinant of if we are charging.
	writeFileStrict(chargeStartThresholdPath, "50")

	s.Log("Setting end threshold high so that we should be charging")
	writeFileStrict(chargeModePath, "Custom")
	writeFileStrict(chargeEndThresholdPath, "100")
	verifyCharging(true)

	s.Log("Setting end threshold low so that we should stop charging")
	writeFileStrict(chargeModePath, "Custom")
	writeFileStrict(chargeEndThresholdPath, "55")
	verifyCharging(false)

	s.Log("Setting end threshold high so that we should be charging")
	writeFileStrict(chargeModePath, "Custom")
	writeFileStrict(chargeEndThresholdPath, "100")
	verifyCharging(true)
}
