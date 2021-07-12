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
	"chromiumos/tast/errors"
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
		Func: AdvancedBatteryChargeMode,
		Desc: "Tests DeviceAdvancedBatteryCharge policies that maximize battery health",
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

// AdvancedBatteryChargeMode verifies DeviceAdvancedBatteryChargeModeEnabled & DeviceAdvancedBatteryChargeModeDayConfig,
// a power management & its corresponding configuration policy that let users maximize battery health. In advanced charging
// mode the system will use standard charging algorithm during non working hours to maximize battery. During work hours, an
// express charge is used to charge the battery as quick as possible. Setting the policy to disabled or leaving it unset
// keeps advanced battery charge mode off.
func AdvancedBatteryChargeMode(ctx context.Context, s *testing.State) {
	const (
		minLevel = 91
		maxLevel = 94
	)
	mockDate := time.Date(2001, time.January, 1, 12, 0, 0, 0, time.UTC) // 12 pm, Monday, January 1, 2001.

	pxy, err := servo.NewProxy(ctx, s.RequiredVar("servo"), s.DUT().KeyFile(), s.DUT().KeyDir())
	if err != nil {
		s.Fatal("Failed to establish proxy with servo: ", err)
	}

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to establish gRPC channel with DUT: ", err)
	}
	defer cl.Close(ctx)

	// Putting battery within testable range.
	_, err = power.EnsureBatteryPercentage(ctx, cl, pxy.Servo(), minLevel, maxLevel)
	if err != nil {
		s.Fatalf("Failed to ensure battery percentage within %d to %d: %v", minLevel, maxLevel, err)
	}

	if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, s.DUT()); err != nil {
		s.Error("Failed to reset TPM: ", err)
	}
	if cl, err = rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros"); err != nil {
		s.Fatal("Failed to establish gRPC channel with DUT: ", err)
	}
	defer cl.Close(ctx)

	// advanced battery charge config is dependent on DUT internal RTC. For stable testing a mock-time needs to be applied.
	wc := ws.NewWilcoServiceClient(cl.Conn)
	if _, err := wc.SetRTC(ctx, &timestamp.Timestamp{
		Seconds: mockDate.Unix(),
	}); err != nil {
		s.Fatalf("Failed to apply mock time to %s: %v", mockDate.Format(time.RFC3339), err)
	}
	defer func(ctx context.Context) {
		if _, err := wc.ResetRTC(ctx, &empty.Empty{}); err != nil {
			testing.ContextLog(ctx, "Failed to reset EC RTC: ", err)
		}
	}(ctx)

	// Connecting DUT with power supply.
	if err := pxy.Servo().SetPDRole(ctx, servo.PDRoleSrc); err != nil {
		s.Fatal("Failed to switch servo_pd_role to src: ", err)
	}

	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	powc := pws.NewPowerServiceClient(cl.Conn)
	pc := ps.NewPolicyServiceClient(cl.Conn)

	pJSON, err := json.Marshal(fakedms.NewPolicyBlob())
	if err != nil {
		s.Fatal("Failed to serialize policies: ", err)
	}

	if _, err := pc.EnrollUsingChrome(ctx, &ps.EnrollUsingChromeRequest{
		PolicyJson: pJSON,
	}); err != nil {
		s.Fatal("Failed to apply policy through fakedms: ", err)
	}
	defer pc.StopChromeAndFakeDMS(ctx, &empty.Empty{})

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
			policies:     []policy.Policy{&policy.DeviceAdvancedBatteryChargeModeEnabled{Val: false}},
			wantOnAc:     true,
			wantCharging: true,
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
			wantOnAc:     true,
			wantCharging: false,
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
			wantOnAc:     true,
			wantCharging: true,
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
			wantOnAc:     true,
			wantCharging: false,
		},
	} {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			// Updating policy blob.
			pb := fakedms.NewPolicyBlob()
			pb.AddPolicies(tc.policies)

			pJSON, err := json.Marshal(pb)
			if err != nil {
				s.Fatal("Failed to serialize policies: ", err)
			}

			if _, err := pc.UpdatePolicies(ctx, &ps.UpdatePoliciesRequest{
				PolicyJson: pJSON,
			}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			if err := testing.Poll(ctx, func(ctx context.Context) error {
				// Checking current battery and power state.
				status, err := powc.BatteryStatus(ctx, &empty.Empty{})
				if err != nil {
					testing.PollBreak(errors.Wrap(err, "failed to get battery status"))
				}

				if status.OnAc != tc.wantOnAc || status.Charging != tc.wantCharging {
					return errors.Wrapf(nil, "want: charging_state-%v & ac_supply- %v, got: charging_state-%v & ac_supply- %v",
						tc.wantCharging, tc.wantOnAc, status.Charging, status.OnAc)
				}

				return nil
			}, &testing.PollOptions{
				Interval: time.Second,
				Timeout:  30 * time.Second,
			}); err != nil {
				s.Error("Subtest failed: ", err)
			}
		})
	}
}
