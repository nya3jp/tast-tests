// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/power/charge"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DeviceBatteryChargeMode,
		Desc: "Tests the DeviceBatteryCharge policies that extend battery life",
		Contacts: []string{
			"bisakhmondal00@gmail.com", // test author
			"lamzin@google.com",        // wilco_dtc_supportd maintainer
			"chromeos-wilco@google.com",
		},
		SoftwareDeps: []string{"wilco", "chrome"},
		Timeout:      25 * time.Minute,
		Attr:         []string{"group:mainline", "informational"},
		HardwareDeps: hwdep.D(hwdep.Battery()),
		Vars:         []string{"servo"},
		Fixture:      "chromeEnrolledLoggedIn",
	})
}

// DeviceBatteryChargeMode verifies DeviceBatteryCharge policies, a group of power management policies, dynamically controls
// battery charging state to minimize stress and wear-out due to the exposure of rapid charging/discharging cycles and extend
// the battery life. If the policy is set then battery charge mode will be applied on the DUT. Leaving the policy unset applies
// the standard battery charge mode.
//
// The Policy takes either one of the five values ranging from 1 to 5:
// 1 = Fully charge battery at a standard rate.
// 2 = Charge battery using fast charging technology.
// 3 = Charge battery for devices that are primarily connected to an external power source.
// 4 = Adaptive charge battery based on battery usage pattern.
// 5 = Charge battery while it is within a fixed range.
// If Custom battery charge mode (5) is selected, then DeviceBatteryChargeCustomStartCharging and
// DeviceBatteryChargeCustomStopCharging values need to be specified alongside.
func DeviceBatteryChargeMode(ctx context.Context, s *testing.State) {
	const (
		// Minimum battery percentage requires in DUT for successful sub testing.
		// Subtest "custom_charge_outside_range" doesn't charge DUT if the battery is above 80%.
		minLevel = 81
		// DUT generally has three power state [Charging, Full & Discharging] and we are interested in checking the
		// Discharging state while connecting it to a constant power supply. That's why it is logical to keep
		// a reasonable buffer from the Full state (100%) to have a proper distinction during sub testing.
		maxLevel = 95
	)

	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	// Shorten deadline to leave time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	srvo, err := servo.NewDirect(ctx, s.RequiredVar("servo"))
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer srvo.Close(cleanupCtx)

	// Putting battery within testable range.
	if err := charge.EnsureBatteryWithinRange(ctx, cr, srvo, minLevel, maxLevel); err != nil {
		s.Fatalf("Failed to ensure battery percentage within %d%% to %d%%: %v", minLevel, maxLevel, err)
	}

	// Connect DUT with power supply.
	if err := srvo.SetPDRole(ctx, servo.PDRoleSrc); err != nil {
		s.Fatal("Failed to switch servo_pd_role to src: ", err)
	}

	for _, tc := range []struct {
		name            string
		policies        []policy.Policy
		wantOnAc        bool
		wantDischarging bool
	}{
		{
			name:            "unset",
			policies:        []policy.Policy{},
			wantOnAc:        true,
			wantDischarging: false,
		},
		{
			name: "standard_charge",
			policies: []policy.Policy{&policy.DeviceBatteryChargeMode{
				Val: 1,
			}},
			wantOnAc:        true,
			wantDischarging: false,
		},
		{
			name: "fast_charge",
			policies: []policy.Policy{&policy.DeviceBatteryChargeMode{
				Val: 2,
			}},
			wantOnAc:        true,
			wantDischarging: false,
		},
		{
			name: "primarily_ac",
			policies: []policy.Policy{&policy.DeviceBatteryChargeMode{
				Val: 3,
			}},
			wantOnAc:        true,
			wantDischarging: false,
		},
		{
			name: "adaptive_charge",
			policies: []policy.Policy{&policy.DeviceBatteryChargeMode{
				Val: 4,
			}},
			wantOnAc:        true,
			wantDischarging: false,
		},
		{
			name: "custom_charge_outside_range",
			policies: []policy.Policy{&policy.DeviceBatteryChargeMode{
				Val: 5,
			},
				&policy.DeviceBatteryChargeCustomStartCharging{Val: 40},
				&policy.DeviceBatteryChargeCustomStopCharging{Val: 80},
			},
			wantOnAc:        true,
			wantDischarging: true,
		},
		{
			name: "custom_charge_within_range",
			policies: []policy.Policy{&policy.DeviceBatteryChargeMode{
				Val: 5,
			},
				&policy.DeviceBatteryChargeCustomStartCharging{Val: 40},
				&policy.DeviceBatteryChargeCustomStopCharging{Val: 100},
			},
			wantOnAc:        true,
			wantDischarging: false,
		},
	} {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndRefresh(ctx, fdms, cr, tc.policies); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			if err := testing.Poll(ctx, func(ctx context.Context) error {
				// Checking current battery status and power state.
				status, err := power.GetStatus(ctx)
				if err != nil {
					return testing.PollBreak(errors.Wrap(err, "failed to get battery status"))
				}

				if status.LinePowerConnected != tc.wantOnAc {
					return errors.Errorf("unexpected AC supply: want %v; got %v", tc.wantOnAc, status.LinePowerConnected)
				}
				if status.BatteryDischarging != tc.wantDischarging {
					return errors.Errorf("unexpected discharging state: want %v; got %v", tc.wantDischarging, status.BatteryDischarging)
				}

				return nil
			}, &testing.PollOptions{
				Timeout: 30 * time.Second,
			}); err != nil {
				s.Error("Failed to wait for expected battery state: ", err)
			}
		})
	}
}
