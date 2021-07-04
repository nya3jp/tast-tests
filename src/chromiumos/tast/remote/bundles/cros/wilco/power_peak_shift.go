// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"encoding/json"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/policyutil"
	"chromiumos/tast/remote/power"
	"chromiumos/tast/remote/servo"
	"chromiumos/tast/rpc"
	ps "chromiumos/tast/services/cros/policy"
	pws "chromiumos/tast/services/cros/power"
	ws "chromiumos/tast/services/cros/wilco"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PowerPeakShift,
		Desc: "Tests DevicePowerPeakShiftEnabled policy that minimizes alternating current usage during peak hours",
		Contacts: []string{
			"bisakhmondal00@gmail.com", // test author
			"lamzin@chromium.org",      // wilco_dtc_supportd maintainer
			"chromeos-wilco@google.com",
		},
		Attr:         []string{"group:enrollment"},
		VarDeps:      []string{"servo"},
		SoftwareDeps: []string{"wilco", "chrome", "reboot"},
		HardwareDeps: hwdep.D(hwdep.Battery()),
		Timeout:      20 * time.Minute,
		ServiceDeps:  []string{"tast.cros.power.PowerService", "tast.cros.wilco.WilcoService", "tast.cros.policy.PolicyService"},
	})
}

// PowerPeakShift verifies DevicePowerPeakShiftEnabled Policy which is a power-saving
// policy that minimizes alternating current usage during peak times. If the policy is set
// then DevicePowerPeakShiftBatteryThreshold & DevicePowerPeakShiftDayConfig policies are also set
// along with peak shift. If it is unset or disabled peak shift is disabled.
//
// Brief: Policy DevicePowerPeakShiftDayConfig has "start_time", "end_time" and "charge_start_time" fields.
// When DUT is above battery threshold set through DevicePowerPeakShiftBatteryThreshold policy,
// current time is in between "start_time" and "end_time" DUT uses battery even if it is plugged to AC.
// After the "end_time" period, DUT runs on AC till "start_time_charge" time and doesn't charge battery.
func PowerPeakShift(ctx context.Context, s *testing.State) {
	const (
		minLevel = 16
		maxLevel = 95
		mockDate = "1/1/01 12:00:00" // 12pm, Monday, January 1, 2001
	)

	if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, s.DUT()); err != nil {
		s.Error("Failed to reset TPM during powerwash: ", err)
	}

	pxy, err := servo.NewProxy(ctx, s.RequiredVar("servo"), s.DUT().KeyFile(), s.DUT().KeyDir())
	if err != nil {
		s.Fatal("Failed to establish proxy with servo: ", err)
	}

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to establish grpc channel with DUT: ", err)
	}
	defer cl.Close(ctx)

	// Putting battery within certain power range
	if err := power.EnsureBatteryPercentage(ctx, cl, pxy.Servo(), minLevel, maxLevel); err != nil {
		s.Fatal("Failed to ensure battery percentage with a range: ", err)
	}

	// Applying mock time to DUT, Monday, January 1, 2001, Monday
	wc := ws.NewWilcoServiceClient(cl.Conn)

	if _, err := wc.SetRTC(ctx, &ws.SetRTCRequest{
		Datetime: mockDate,
	}); err != nil {
		s.Fatalf("Unable to set mock time to %s: %v", mockDate, err)
	}
	defer func(ctx context.Context) {
		if _, err := wc.ResetRTC(ctx, &empty.Empty{}); err != nil {
			testing.ContextLog(ctx, "Failed to reset EC's Real Time Clock: ", err)
		}
	}(ctx)

	// Connecting DUT with power supply
	if err := pxy.Servo().SetPDRole(ctx, servo.PDRoleSrc); err != nil {
		s.Fatal("Failed to switch servo_pd_role to src: ", err)
	}

	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	powc := pws.NewPowerServiceClient(cl.Conn)
	pc := ps.NewPolicyServiceClient(cl.Conn)
	defer pc.StopChromeAndFakeDMS(ctx, &empty.Empty{})

	emptypb := fakedms.NewPolicyBlob()

	pJSON, err := json.Marshal(emptypb)
	if err != nil {
		s.Fatal("Failed to serialize policies: ", err)
	}

	if _, err := pc.EnrollUsingChrome(ctx, &ps.EnrollUsingChromeRequest{
		PolicyJson: pJSON,
	}); err != nil {
		s.Fatal("Failed to apply policy through fakedms: ", err)
	}

	// Iterate over test cases and verify expected behavior.
	for _, tc := range []struct {
		name     string
		policies []policy.Policy
		charging bool
		onAc     bool
	}{
		{
			name:     "not_set",
			policies: []policy.Policy{&policy.DevicePowerPeakShiftEnabled{Stat: policy.StatusUnset}},
			charging: true,
			onAc:     true,
		},
		{
			name:     "disabled",
			policies: []policy.Policy{&policy.DevicePowerPeakShiftEnabled{Val: false}},
			charging: true,
			onAc:     true,
		},
		{
			name: "enabled_before_start_time_config",
			policies: []policy.Policy{&policy.DevicePowerPeakShiftEnabled{Val: true},
				&policy.DevicePowerPeakShiftBatteryThreshold{Val: 15},
				&policy.DevicePowerPeakShiftDayConfig{
					Stat: 0,
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
			charging: true,
			onAc:     true,
		},
		{
			name: "enabled_within_start_end_config",
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
			charging: false,
			onAc:     false,
		},
		{
			name: "enabled_after_end_before_charge_start_config",
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
			charging: false,
			onAc:     true,
		},
		{
			name: "enabled_after_end_config",
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
			charging: true,
			onAc:     true,
		},
	} {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {

			pb := fakedms.NewPolicyBlob()

			pb.AddPolicies(tc.policies)

			pJSON, err := json.Marshal(pb)
			if err != nil {
				s.Fatal("Failed to serialize policies: ", err)
			}

			if _, err := pc.UpdatePolicies(ctx, &ps.UpdatePoliciesRequest{
				PolicyJson: pJSON,
			}); err != nil {
				s.Fatal("Failed to update peak shift policy: ", err)
			}

			// Checking current battery and power state
			status, err := powc.BatteryStatus(ctx, &empty.Empty{})
			if err != nil {
				s.Fatal("Failed to get battery status: ", err)
			}

			if status.OnAc != tc.onAc || status.Charging != tc.charging {
				s.Errorf("want: charging_state-%v & ac_supply- %v, got: charging_state-%v & ac_supply- %v",
					tc.charging, tc.onAc, status.Charging, status.OnAc)
			}
		})
	}
}
