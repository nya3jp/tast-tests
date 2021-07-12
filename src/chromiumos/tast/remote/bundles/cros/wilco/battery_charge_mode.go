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
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: BatteryChargeMode,
		Desc: "Tests DeviceBatteryChargeMode policy that minimizes battery wear-out over time",
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
		ServiceDeps:  []string{"tast.cros.power.PowerService", "tast.cros.policy.PolicyService"},
	})
}

// BatteryChargeMode verifies DeviceBatteryChargeMode, a power management policy which dynamically controls battery
// charging state to minimize wear-out due to the exposure of prolonged stress and extends the battery life. If the
// policy is set then battery charge mode will be applied on the DUT. Leaving the policy unset applies the standard
// battery charge mode.
//
// The Policy takes either one of the five values ranging from 1 to 5
// 1 = Fully charge battery at a standard rate.
// 2 = Charge battery using fast charging technology.
// 3 = Charge battery for devices that are primarily connected to an external power source.
// 4 = Adaptive charge battery based on battery usage pattern.
// 5 = Charge battery while it is within a fixed range.
// If Custom battery charge mode (5) is selected, then DeviceBatteryChargeCustomStartCharging and
// DeviceBatteryChargeCustomStopCharging need to be specified alongside.
func BatteryChargeMode(ctx context.Context, s *testing.State) {
	const (
		minLevel = 87
		maxLevel = 95
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

	// Connecting DUT with power supply
	if err := pxy.Servo().SetPDRole(ctx, servo.PDRoleSrc); err != nil {
		s.Fatal("Failed to switch servo_pd_role to src: ", err)
	}

	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	powc := pws.NewPowerServiceClient(cl.Conn)
	pc := ps.NewPolicyServiceClient(cl.Conn)

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
			name: "standard_charge",
			policies: []policy.Policy{&policy.DeviceBatteryChargeMode{
				Val: 1,
			}},
			onAc:     true,
			charging: true,
		},
		{
			name: "fast_charge",
			policies: []policy.Policy{&policy.DeviceBatteryChargeMode{
				Val: 2,
			}},
			onAc:     true,
			charging: true,
		},
		{
			name: "primarily_ac",
			policies: []policy.Policy{&policy.DeviceBatteryChargeMode{
				Val: 3,
			}},
			onAc:     true,
			charging: true,
		},
		{
			name: "adaptive_charge",
			policies: []policy.Policy{&policy.DeviceBatteryChargeMode{
				Val: 4,
			}},
			onAc:     true,
			charging: true,
		},
		{
			name: "custom_charge_outside_range",
			policies: []policy.Policy{&policy.DeviceBatteryChargeMode{
				Val: 5,
			},
				&policy.DeviceBatteryChargeCustomStartCharging{Val: 40},
				&policy.DeviceBatteryChargeCustomStopCharging{Val: 75},
			},
			onAc:     true,
			charging: false,
		},
		{
			name: "custom_charge_within_range",
			policies: []policy.Policy{&policy.DeviceBatteryChargeMode{
				Val: 5,
			},
				&policy.DeviceBatteryChargeCustomStartCharging{Val: 40},
				&policy.DeviceBatteryChargeCustomStopCharging{Val: 100},
			},
			onAc:     true,
			charging: true,
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
				s.Fatal("Failed to update battery charge policy: ", err)
			}
			testing.Sleep(ctx, 15*time.Second)

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
