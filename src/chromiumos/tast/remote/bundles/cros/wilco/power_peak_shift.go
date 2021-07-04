// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"encoding/json"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/golang/protobuf/ptypes/timestamp"

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
		Desc: "A variant of power policy that verifies charging behaviours on DUT. It is currently supported for wilco devices",
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
	)

	waitctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if err := policyutil.EnsureTPMAndSystemStateAreReset(waitctx, s.DUT()); err != nil {
		s.Error("Failed to reset TPM during powerwash: ", err)
	}
	waitctx, cancel = context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if err := s.DUT().WaitConnect(waitctx); err != nil { // Ideally this step is redundant
		s.Fatal("Failed to reconnect the device: ", err)
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
	defer func(ctx context.Context) {
		if _, err := wc.ResetRTC(ctx, &empty.Empty{}); err != nil {
			testing.ContextLog(ctx, "Failed to reset EC's Real Time Clock: ", err)
		}
	}(ctx)
	if _, err := wc.SetRTC(ctx, &timestamp.Timestamp{
		Seconds: time.Date(2001, time.January, 1, 12, 0, 0, 0, time.UTC).UTC().Unix(),
	}); err != nil {
		s.Fatal("Unable to set mock time to January 1, 2001, Monday: ", err)
	}

	// Connecting DUT with power supply
	if err := pxy.Servo().SetPDRole(ctx, servo.PDRoleSrc); err != nil {
		s.Fatal("Failed to switch servo_pd_role to src: ", err)
	}

	ctx, cancel = ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	// Iterate over test cases and verify expected behavior.
	for _, tc := range []struct {
		name     string
		base     *policy.DevicePowerPeakShiftEnabled
		thresh   *policy.DevicePowerPeakShiftBatteryThreshold
		config   *policy.DevicePowerPeakShiftDayConfig
		expected pws.PowerStatus
	}{
		{
			name:     "disabled",
			base:     &policy.DevicePowerPeakShiftEnabled{Val: false},
			thresh:   &policy.DevicePowerPeakShiftBatteryThreshold{Val: 0},
			config:   &policy.DevicePowerPeakShiftDayConfig{},
			expected: pws.PowerStatus_ON_AC_AND_CHARGING,
		},
		{
			name:     "not_set",
			base:     &policy.DevicePowerPeakShiftEnabled{Stat: policy.StatusSet},
			thresh:   &policy.DevicePowerPeakShiftBatteryThreshold{Val: 0},
			config:   &policy.DevicePowerPeakShiftDayConfig{},
			expected: pws.PowerStatus_ON_AC_AND_CHARGING,
		},
		{
			name:   "enabled_before_start_time_config",
			base:   &policy.DevicePowerPeakShiftEnabled{Val: true},
			thresh: &policy.DevicePowerPeakShiftBatteryThreshold{Val: 15},
			config: &policy.DevicePowerPeakShiftDayConfig{
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
			expected: pws.PowerStatus_ON_AC_AND_CHARGING,
		},
		{
			name:   "enabled_within_start_end_config",
			base:   &policy.DevicePowerPeakShiftEnabled{Val: true},
			thresh: &policy.DevicePowerPeakShiftBatteryThreshold{Val: 15},
			config: &policy.DevicePowerPeakShiftDayConfig{
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
			expected: pws.PowerStatus_NOT_ON_AC_AND_NOT_CHARGING,
		},
		{
			name:   "enabled_after_end_before_charge_start_config",
			base:   &policy.DevicePowerPeakShiftEnabled{Val: true},
			thresh: &policy.DevicePowerPeakShiftBatteryThreshold{Val: 15},
			config: &policy.DevicePowerPeakShiftDayConfig{
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
			expected: pws.PowerStatus_ON_AC_AND_NOT_CHARGING,
		},
		{
			name:   "enabled_after_end_config",
			base:   &policy.DevicePowerPeakShiftEnabled{Val: true},
			thresh: &policy.DevicePowerPeakShiftBatteryThreshold{Val: 15},
			config: &policy.DevicePowerPeakShiftDayConfig{
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
			expected: pws.PowerStatus_ON_AC_AND_CHARGING,
		},
	} {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			pc := ps.NewPolicyServiceClient(cl.Conn)
			pb := fakedms.NewPolicyBlob()

			pb.AddPolicies([]policy.Policy{tc.base, tc.thresh, tc.config})

			pJSON, err := json.Marshal(pb)
			if err != nil {
				s.Fatal("Failed to serialize policies: ", err)
			}

			if _, err := pc.EnrollUsingChrome(ctx, &ps.EnrollUsingChromeRequest{
				PolicyJson: pJSON,
			}); err != nil {
				s.Fatal("Failed to apply policy through fakedms: ", err)
			}

			defer pc.StopChromeAndFakeDMS(ctx, &empty.Empty{})

			// Checking current battery and power state
			powc := pws.NewPowerServiceClient(cl.Conn)
			status, err := powc.BatteryStatus(ctx, &empty.Empty{})
			if err != nil {
				s.Fatal("Failed to get battery status: ", err)
			}

			if status.Status != tc.expected {
				s.Errorf("Expected: %s, Received: %s", tc.expected, status.Status)
			}
		})
	}
}
