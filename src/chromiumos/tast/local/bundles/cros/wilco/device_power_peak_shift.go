// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

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
	"chromiumos/tast/local/power/battery"
	"chromiumos/tast/local/rtc"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DevicePowerPeakShift,
		Desc: "Tests for DevicePowerPeakShift policies that minimize alternating current (AC) usage during peak hours",
		Contacts: []string{
			"bisakhmondal00@gmail.com", // test author
			"lamzin@google.com",        // wilco_dtc_supportd maintainer
			"chromeos-wilco@google.com",
		},
		SoftwareDeps: []string{"wilco", "reboot", "chrome"},
		Timeout:      25 * time.Minute,
		Attr:         []string{"group:mainline", "informational"},
		HardwareDeps: hwdep.D(hwdep.Battery()),
		Vars:         []string{"servo"},
		Fixture:      "chromeEnrolledLoggedIn",
	})
}

// DevicePowerPeakShift verifies DevicePowerPeakShift policy groups (power saving policy)
// that minimize alternating current usage during peak times. [ DevicePowerPeakShiftEnabled
// requires DevicePowerPeakShiftBatteryThreshold & DevicePowerPeakShiftDayConfig
// to be set. Leaving them unset keeps peak shift off. ]
//
// Brief: Policy DevicePowerPeakShiftDayConfig has "start_time", "end_time" and "charge_start_time" fields.
// When DUT is above battery threshold set through DevicePowerPeakShiftBatteryThreshold policy,
// current time is in between "start_time" and "end_time" DUT uses battery even if it is plugged to AC.
// Even after the "end_time" period, DUT runs on AC till "charge_start_time" but doesn't charge the battery.
func DevicePowerPeakShift(ctx context.Context, s *testing.State) {
	const (
		minLevel = 16
		maxLevel = 94
	)
	mockDate := time.Date(2001, time.January, 1, 12, 0, 0, 0, time.UTC) // 12 pm, Monday, January 1, 2001.

	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	pxy, err := servo.NewProxy(ctx, s.RequiredVar("servo"), servo.KeyFile, servo.KeyDir)
	if err != nil {
		s.Fatal("Failed to establish proxy with servo: ", err)
	}
	// Putting battery within testable range.
	if err := battery.EnsureBatteryWithinRange(ctx, cr, pxy.Servo(), minLevel, maxLevel); err != nil {
		s.Fatalf("Failed to ensure battery percentage within %d to %d: %v", minLevel, maxLevel, err)
	}

	// Peak shift config is a time dependent policy, depends on DUT internal RTC. For stable testing a mock-time needs to be applied.
	ecRTC := rtc.RTC{DevName: "rtc1", LocalTime: true, NoAdjfile: true}
	restore, err := ecRTC.MockECRTC(ctx, mockDate)
	if err != nil {
		s.Fatalf("Failed to update EC RTC to the mock time %s: %v", mockDate.Format(time.RFC3339), err)
	}
	defer func(ctx context.Context) {
		if err := restore(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to restore EC RTC: ", err)
		}
	}(ctx)

	// Connecting DUT with power supply.
	if err := pxy.Servo().SetPDRole(ctx, servo.PDRoleSrc); err != nil {
		s.Fatal("Failed to switch servo_pd_role to src: ", err)
	}

	ctx, cancel := ctxutil.Shorten(ctx, 20*time.Second)
	defer cancel()

	// Iterate over test cases and verify expected behavior.
	for _, tc := range []struct {
		name         string
		policies     []policy.Policy
		wantOnAc     bool
		wantCharging bool
	}{
		{
			name:         "unset",
			policies:     []policy.Policy{},
			wantOnAc:     true,
			wantCharging: true,
		},
		{
			name:         "disabled",
			policies:     []policy.Policy{&policy.DevicePowerPeakShiftEnabled{Val: false}},
			wantOnAc:     true,
			wantCharging: true,
		},
		{
			name: "enabled_before_start",
			policies: []policy.Policy{&policy.DevicePowerPeakShiftEnabled{Val: true},
				&policy.DevicePowerPeakShiftBatteryThreshold{Val: 15},
				&policy.DevicePowerPeakShiftDayConfig{
					Val: &policy.DevicePowerPeakShiftDayConfigValue{
						Entries: []*policy.DevicePowerPeakShiftDayConfigValueEntries{
							{
								Day: "MONDAY",
								StartTime: &policy.RefTime{
									Hour:   21,
									Minute: 0,
								},
								EndTime: &policy.RefTime{
									Hour:   22,
									Minute: 0,
								},
								ChargeStartTime: &policy.RefTime{
									Hour:   23,
									Minute: 0,
								},
							},
						},
					},
				},
			},
			wantOnAc:     true,
			wantCharging: true,
		},
		{
			name: "enabled_before_end",
			policies: []policy.Policy{
				&policy.DevicePowerPeakShiftEnabled{Val: true},
				&policy.DevicePowerPeakShiftBatteryThreshold{Val: 15},
				&policy.DevicePowerPeakShiftDayConfig{
					Val: &policy.DevicePowerPeakShiftDayConfigValue{
						Entries: []*policy.DevicePowerPeakShiftDayConfigValueEntries{
							{
								Day: "MONDAY",
								StartTime: &policy.RefTime{
									Hour:   1,
									Minute: 0,
								},
								EndTime: &policy.RefTime{
									Hour:   22,
									Minute: 0,
								},
								ChargeStartTime: &policy.RefTime{
									Hour:   23,
									Minute: 0,
								},
							},
						},
					},
				},
			},
			wantOnAc:     false,
			wantCharging: false,
		},
		{
			name: "enabled_before_charge_start",
			policies: []policy.Policy{&policy.DevicePowerPeakShiftEnabled{Val: true},
				&policy.DevicePowerPeakShiftBatteryThreshold{Val: 15},
				&policy.DevicePowerPeakShiftDayConfig{
					Val: &policy.DevicePowerPeakShiftDayConfigValue{
						Entries: []*policy.DevicePowerPeakShiftDayConfigValueEntries{
							{
								Day: "MONDAY",
								StartTime: &policy.RefTime{
									Hour:   1,
									Minute: 0,
								},
								EndTime: &policy.RefTime{
									Hour:   5,
									Minute: 0,
								},
								ChargeStartTime: &policy.RefTime{
									Hour:   23,
									Minute: 0,
								},
							},
						},
					},
				},
			},
			wantOnAc:     true,
			wantCharging: false,
		},
		{
			name: "enabled_after_charge_start",
			policies: []policy.Policy{&policy.DevicePowerPeakShiftEnabled{Val: true},
				&policy.DevicePowerPeakShiftBatteryThreshold{Val: 15},
				&policy.DevicePowerPeakShiftDayConfig{
					Val: &policy.DevicePowerPeakShiftDayConfigValue{
						Entries: []*policy.DevicePowerPeakShiftDayConfigValueEntries{
							{
								Day: "MONDAY",
								StartTime: &policy.RefTime{
									Hour:   1,
									Minute: 0,
								},
								EndTime: &policy.RefTime{
									Hour:   5,
									Minute: 0,
								},
								ChargeStartTime: &policy.RefTime{
									Hour:   6,
									Minute: 0,
								},
							},
						},
					},
				},
			},
			wantOnAc:     true,
			wantCharging: true,
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
					testing.PollBreak(errors.Wrap(err, "failed to get battery status"))
				}

				if status.LinePowerConnected != tc.wantOnAc || status.BatteryDischarging == tc.wantCharging {
					return errors.Wrapf(nil, "want: charging_state-%v & ac_supply- %v, got: charging_state-%v & ac_supply- %v",
						tc.wantCharging, tc.wantOnAc, status.LinePowerConnected, status.BatteryDischarging)
				}

				return nil
			}, &testing.PollOptions{
				Interval: time.Second,
				Timeout:  30 * time.Second,
			}); err != nil {
				s.Errorf("Subtest failed %s: %v", tc.name, err)
			}
		})
	}
}
