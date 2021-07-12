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
		Func: AdvancedBatteryChargeMode,
		Desc: "Tests DeviceAdvancedBatteryChargeModeEnabled policy that maximizes battery health",
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

// AdvancedBatteryChargeMode verifies DeviceAdvancedBatteryChargeModeEnabled, a power management
// policy that lets users maximize battery health. In advanced charging mode the system will use
// standard charging algorithm during non working hours to maximize battery. During work hours, an
// express charge is used to charge the battery as quick as possible. Setting the policy to
// disabled or leaving it unset keeps advanced battery charge mode off.
func AdvancedBatteryChargeMode(ctx context.Context, s *testing.State) {
	const (
		minLevel = 91
		maxLevel = 95
		mockDate = "1/1/01 12:00:00" // 12 pm, Monday, January 1, 2001
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

	// Applying mock time to DUT, Monday, January 1, 2001
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

	pc := ps.NewPolicyServiceClient(cl.Conn)
	powc := pws.NewPowerServiceClient(cl.Conn)

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
	defer pc.StopChromeAndFakeDMS(ctx, &empty.Empty{})

	// Iterate over test cases and verify expected behavior.
	for _, tc := range []struct {
		name     string
		policies []policy.Policy
		onAc     bool
		charging bool
	}{
		{
			name:     "not_set",
			policies: []policy.Policy{&policy.DeviceAdvancedBatteryChargeModeEnabled{Stat: policy.StatusUnset}},
			onAc:     true,
			charging: true,
		},
		{
			name:     "disabled",
			policies: []policy.Policy{&policy.DeviceAdvancedBatteryChargeModeEnabled{Val: false}},
			onAc:     true,
			charging: true,
		},
		{
			name: "enabled_before_start_time_config",
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
			onAc:     true,
			charging: false,
		},
		{
			name: "enabled_within_start_end_config",
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
			onAc:     true,
			charging: true,
		},
		{
			name: "enabled_after_end_config",
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
			onAc:     true,
			charging: false,
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
				s.Fatal("Failed to update advanced battery charge policy: ", err)
			}
			testing.Sleep(ctx, 15*time.Second)

			// Checking current battery and power state
			status, err := powc.BatteryStatus(ctx, &empty.Empty{})
			if err != nil {
				s.Fatal("Failed to get battery status: ", err)
			}

			if status.OnAc != tc.onAc || status.Charging != tc.charging {
				s.Errorf("want: charging state-%v & ac supply- %v, got: charging state-%v & ac supply- %v",
					tc.charging, tc.onAc, status.Charging, status.OnAc)
			}
		})
	}
}
