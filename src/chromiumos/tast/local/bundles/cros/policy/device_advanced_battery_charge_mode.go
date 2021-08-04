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
	"chromiumos/tast/local/rtc"
	"chromiumos/tast/local/wilco"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DeviceAdvancedBatteryChargeMode,
		Desc: "Tests the DeviceAdvancedBatteryCharge policies that maximize battery health",
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

// DeviceAdvancedBatteryChargeMode verifies DeviceAdvancedBatteryCharge policy group (power saving policy)
// that let users maximize the battery health by using a standard charging algorithm and other techniques
// during non-working hours. During work hours, it uses express charging to charge the battery as quickly
// as possible. Setting the policy to disabled or leaving it unset keeps advanced battery charge mode off.
func DeviceAdvancedBatteryChargeMode(ctx context.Context, s *testing.State) {
	const (
		// Minimum battery percentage requires in the DUT for successful sub testing.
		// Subtest "enabled_after_end" (time is outside of the work hours) doesn't charge DUT even when
		// connected to an AC source if the battery is above 90%.
		minLevel = 91
		// DUT generally has three power state [Charging, Full & Discharging] and we are interested in checking the
		// Discharging state while connecting it to a constant power supply. That's why it is logical to keep
		// a reasonable buffer from the Full state (100%) to have a proper distinction during sub testing.
		maxLevel = 95
	)
	mockDate := time.Date(2001, time.January, 1, 12, 0, 0, 0, time.UTC) // 12 pm, Monday, January 1, 2001

	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	// Shorten deadline to leave time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	srvo, err := servo.NewDirect(ctx, s.RequiredVar("servo"))
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer srvo.Close(cleanupCtx)

	// Putting battery within testable range.
	if err := charge.EnsureBatteryWithinRange(ctx, cr, srvo, minLevel, maxLevel); err != nil {
		s.Fatalf("Failed to ensure battery percentage within %d to %d: %v", minLevel, maxLevel, err)
	}

	// Peak shift config is a time dependent policy, depends on DUT internal RTC. For stable testing a mock-time needs to be applied.
	wRTC := wilco.RTC{
		RTC: rtc.RTC{DevName: "rtc1", LocalTime: true, NoAdjfile: true},
	}
	restore, err := wRTC.MockECRTC(ctx, mockDate)
	if err != nil {
		s.Fatalf("Failed to update EC RTC to the mock time %s: %v", mockDate.Format(time.RFC3339), err)
	}

	defer func(ctx context.Context) {
		if err := restore(ctx); err != nil {
			s.Error("Failed to restore EC RTC: ", err)
		}
	}(cleanupCtx)

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
			name:            "disabled",
			policies:        []policy.Policy{&policy.DeviceAdvancedBatteryChargeModeEnabled{Val: false}},
			wantOnAc:        true,
			wantDischarging: false,
		},
		{
			name: "enabled_before_start",
			policies: []policy.Policy{&policy.DeviceAdvancedBatteryChargeModeEnabled{Val: true},
				&policy.DeviceAdvancedBatteryChargeModeDayConfig{
					Val: &policy.DeviceAdvancedBatteryChargeModeDayConfigValue{
						Entries: []*policy.DeviceAdvancedBatteryChargeModeDayConfigValueEntries{{
							Day: "MONDAY",
							ChargeStartTime: &policy.RefTime{
								Hour:   18,
								Minute: 0,
							},
							ChargeEndTime: &policy.RefTime{
								Hour:   22,
								Minute: 0,
							},
						}},
					},
				},
			},
			wantOnAc:        true,
			wantDischarging: true,
		},
		{
			name: "enabled_before_end",
			policies: []policy.Policy{&policy.DeviceAdvancedBatteryChargeModeEnabled{Val: true},
				&policy.DeviceAdvancedBatteryChargeModeDayConfig{
					Val: &policy.DeviceAdvancedBatteryChargeModeDayConfigValue{
						Entries: []*policy.DeviceAdvancedBatteryChargeModeDayConfigValueEntries{{
							Day: "MONDAY",
							ChargeStartTime: &policy.RefTime{
								Hour:   10,
								Minute: 0,
							},
							ChargeEndTime: &policy.RefTime{
								Hour:   18,
								Minute: 0,
							},
						}},
					},
				},
			},
			wantOnAc:        true,
			wantDischarging: false,
		},
		{
			name: "enabled_after_end",
			policies: []policy.Policy{&policy.DeviceAdvancedBatteryChargeModeEnabled{Val: true},
				&policy.DeviceAdvancedBatteryChargeModeDayConfig{
					Val: &policy.DeviceAdvancedBatteryChargeModeDayConfigValue{
						Entries: []*policy.DeviceAdvancedBatteryChargeModeDayConfigValueEntries{{
							Day: "MONDAY",
							ChargeStartTime: &policy.RefTime{
								Hour:   1,
								Minute: 0,
							},
							ChargeEndTime: &policy.RefTime{
								Hour:   2,
								Minute: 0,
							},
						}},
					},
				},
			},
			wantOnAc:        true,
			wantDischarging: true,
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
