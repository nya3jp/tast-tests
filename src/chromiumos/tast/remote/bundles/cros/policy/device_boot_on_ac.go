// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/policyutil"
	"chromiumos/tast/remote/power"
	"chromiumos/tast/rpc"
	ps "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DeviceBootOnAC,
		Desc: "Tests the DeviceBootOnAcEnabled policy that boots up the DUT from shutdown by plugging in a power supply",
		Contacts: []string{
			"bisakhmondal00@gmail.com", // test author
			"lamzin@google.com",        // wilco_dtc_supportd maintainer
			"chromeos-wilco@google.com",
		},
		SoftwareDeps: []string{"wilco", "chrome"},
		Timeout:      25 * time.Minute,
		Attr:         []string{"group:enrollment"},
		ServiceDeps:  []string{"tast.cros.policy.PolicyService"},
		// For boot on AC, we are operating with two servos. One is "servo-v41" to control
		// the charging state (power delivery [pd_role]) of DUT. The other one is the "servo-micro",
		// the micro servo connected to the DUT motherboard that controls the on-off state of DUT.
		Vars: []string{"servo-v41", "servo-micro"},
	})
}

// DeviceBootOnAC verifies DeviceBootOnAcEnabled policy that boots the device from the off state by plugging
// in a power supply. If the policy is disabled or not set, boot on AC is off.
func DeviceBootOnAC(ctx context.Context, s *testing.State) {
	d := s.DUT()

	// isDischarging checks if the DUT is in discharging state.
	isDischarging := func(ctx context.Context) (bool, error) {
		out, err := d.Conn().Command("cat", "/sys/class/power_supply/BAT0/status").Output(ctx)
		if err != nil {
			return false, err
		}
		return strings.TrimSpace(string(out)) == "Discharging", nil
	}

	// Shorten deadline to leave time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Minute)
	defer cancel()

	// Establishing proxy with two different servos.
	pxyMicro, err := servo.NewProxy(ctx, s.RequiredVar("servo-micro"), d.KeyFile(), d.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxyMicro.Close(cleanupCtx)

	pxyV41, err := servo.NewProxy(ctx, s.RequiredVar("servo-v41"), d.KeyFile(), d.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxyV41.Close(cleanupCtx)

	defer func(ctx context.Context) {
		if err := pxyV41.Servo().SetPDRole(ctx, servo.PDRoleSrc); err != nil {
			s.Error("Failed to reset servo power delivery (PD) role to src: ", err)
		}
		if err := power.EnsureDUTisOn(ctx, d, pxyMicro.Servo()); err != nil {
			s.Error("Failed to ensure DUT is powered on: ", err)
		}
		if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, d); err != nil {
			s.Error("Failed to reset TPM: ", err)
		}
	}(cleanupCtx)

	for _, tc := range []struct {
		name       string
		policy     policy.Policy
		wantBootUp bool
	}{
		{
			name:       "unset",
			policy:     &policy.DeviceBootOnAcEnabled{Stat: policy.StatusUnset},
			wantBootUp: false,
		},
		{
			name:       "enabled",
			policy:     &policy.DeviceBootOnAcEnabled{Val: true},
			wantBootUp: true,
		},
		{
			name:       "disabled",
			policy:     &policy.DeviceBootOnAcEnabled{Val: false},
			wantBootUp: false,
		},
	} {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			// For safety purpose, introducing a new cleanup context for device boot up.
			cleanupCtx := ctx
			ctx, cancel := ctxutil.Shorten(ctx, 2*time.Minute)
			defer cancel()

			if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, d); err != nil {
				s.Fatal("Failed to reset TPM: ", err)
			}

			cl, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
			if err != nil {
				s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
			}
			defer cl.Close(ctx)

			pc := ps.NewPolicyServiceClient(cl.Conn)
			pb := fakedms.NewPolicyBlob()
			pb.AddPolicy(tc.policy)

			pJSON, err := json.Marshal(pb)
			if err != nil {
				s.Fatal("Failed to serialize policies: ", err)
			}

			if _, err := pc.EnrollUsingChrome(ctx, &ps.EnrollUsingChromeRequest{
				PolicyJson: pJSON,
			}); err != nil {
				s.Fatal("Failed to enroll using chrome: ", err)
			}

			// Cutting off the power supply.
			if err := pxyV41.Servo().SetPDRole(ctx, servo.PDRoleSnk); err != nil {
				s.Fatal("Failed to cut-off power supply: ", err)
			}

			// Ensuring DUT actually has started discharging. Using polling to tackle
			// the delay of the three power states in DUT i.e. Charging to Unknown to Discharging.
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				discharging, err := isDischarging(ctx)
				if err != nil {
					return testing.PollBreak(err)
				}
				if !discharging {
					return errors.New("device is not in discharging state")
				}
				return nil
			}, &testing.PollOptions{
				Timeout:  10 * time.Second,
				Interval: time.Second,
			}); err != nil {
				s.Fatal("Failed to wait for device discharging state: ", err)
			}

			// Powering off DUT and ensuring DUT is unreachable.
			if err := pxyMicro.Servo().KeypressWithDuration(ctx, servo.PowerKey, servo.DurLongPress); err != nil {
				s.Fatal("Failed to power off DUT: ", err)
			}

			s.Log("Waiting for DUT to become unreachable")
			if err := d.WaitUnreachable(ctx); err != nil {
				s.Fatal("DUT is still reachable while it should not be: ", err)
			}
			s.Log("DUT became unreachable as expected")

			// Even if policy fails, device must be powered on between subtests.
			defer func(ctx context.Context) {
				if err := power.EnsureDUTisOn(ctx, d, pxyMicro.Servo()); err != nil {
					s.Error("Failed to ensure DUT is powered on: ", err)
				}
			}(cleanupCtx)

			// Even after DUT becomes unreachable, it is not completely powered off.
			if err := testing.Sleep(ctx, 15*time.Second); err != nil {
				s.Error("Failed to sleep: ", err)
			}

			// Connecting DUT to the power supply to test the policy behaviour.
			if err := pxyV41.Servo().SetPDRole(ctx, servo.PDRoleSrc); err != nil {
				s.Fatal("Unable to turn on power supply: ", err)
			}

			waitCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			defer cancel()
			if err := d.WaitConnect(waitCtx); err != nil {
				if tc.wantBootUp {
					s.Error("Device didn't boot up while it should be: ", err)
				}
				return
			}

			if !tc.wantBootUp {
				s.Error("Device booted up while it should not be")
			}
		})
	}
}
