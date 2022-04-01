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
	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/policy/dututils"
	"chromiumos/tast/remote/policyutil"
	"chromiumos/tast/rpc"
	ps "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DeviceBootOnAC,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests the DeviceBootOnAcEnabled policy that boots up the DUT from shutdown by plugging in a power supply",
		Contacts: []string{
			"lamzin@google.com", // policy author
			"chromeos-wilco@google.com",
			"bisakhmondal00@gmail.com", // test author
		},
		SoftwareDeps: []string{"wilco", "chrome"},
		Timeout:      30 * time.Minute,
		Attr:         []string{"group:wilco_bve"},
		ServiceDeps:  []string{"tast.cros.policy.PolicyService"},
		// Var "servo" is a ServoV4 Type-C device paired with a Servo Micro via the micro USB port.
		// Servo Micro as usual gets connected to the DUT motherboard debug header and the other cable with
		// a USB-C head is attached to the DUT type C port. Note: both cables must be connected to the DUT.
		Vars: []string{"servo"},
	})
}

// DeviceBootOnAC verifies DeviceBootOnAcEnabled policy that boots the device from the off state by plugging
// in a power supply. If the policy is disabled or not set, boot on AC is off.
func DeviceBootOnAC(ctx context.Context, s *testing.State) {
	d := s.DUT()

	// isDischarging checks if the DUT is in discharging state.
	isDischarging := func(ctx context.Context) (bool, error) {
		out, err := d.Conn().CommandContext(ctx, "cat", "/sys/class/power_supply/BAT0/status").Output()
		if err != nil {
			return false, err
		}
		return strings.TrimSpace(string(out)) == "Discharging", nil
	}

	// Shorten deadline to leave time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Minute)
	defer cancel()

	pxy, err := servo.NewProxy(ctx, s.RequiredVar("servo"), d.KeyFile(), d.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(cleanupCtx)

	defer func(ctx context.Context) {
		if err := pxy.Servo().SetPDRole(ctx, servo.PDRoleSrc); err != nil {
			s.Error("Failed to reset servo power delivery (PD) role to src: ", err)
		}
		if err := dututils.EnsureDUTIsOn(ctx, d, pxy.Servo()); err != nil {
			s.Error("Failed to ensure DUT is powered on: ", err)
		}
		if err := policyutil.EnsureTPMAndSystemStateAreResetRemote(ctx, d); err != nil {
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

			if err := policyutil.EnsureTPMAndSystemStateAreResetRemote(ctx, d); err != nil {
				s.Fatal("Failed to reset TPM: ", err)
			}

			cl, err := rpc.Dial(ctx, d, s.RPCHint())
			if err != nil {
				s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
			}
			defer cl.Close(ctx)

			pc := ps.NewPolicyServiceClient(cl.Conn)
			pb := policy.NewBlob()
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
			if err := pxy.Servo().SetPDRole(ctx, servo.PDRoleSnk); err != nil {
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

			// Even if policy fails, device must be on a power on state in between subtests.
			defer func(ctx context.Context) {
				if err := dututils.EnsureDUTIsOn(ctx, d, pxy.Servo()); err != nil {
					s.Error("Failed to ensure DUT is powered on: ", err)
				}
			}(cleanupCtx)

			// Powering off DUT and ensuring DUT is unreachable.
			if err := pxy.Servo().KeypressWithDuration(ctx, servo.PowerKey, servo.DurLongPress); err != nil {
				s.Fatal("Failed to power off DUT: ", err)
			}

			s.Log("Waiting for DUT to become unreachable")
			if err := d.WaitUnreachable(ctx); err != nil {
				s.Fatal("DUT is still reachable while it should not be: ", err)
			}
			s.Log("DUT became unreachable as expected")

			// Even after DUT becomes unreachable, it is not completely powered off.
			if err := testing.Sleep(ctx, 15*time.Second); err != nil {
				s.Error("Failed to sleep: ", err)
			}

			// Connecting DUT to the power supply to test the policy behaviour.
			if err := pxy.Servo().SetPDRole(ctx, servo.PDRoleSrc); err != nil {
				s.Fatal("Unable to turn on power supply: ", err)
			}

			if tc.wantBootUp {
				waitCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
				defer cancel()
				if err := d.WaitConnect(waitCtx); err != nil {
					s.Error("Failed to wait DUT to be connected: ", err)
				}
			} else {
				if err := dututils.EnsureDisconnected(ctx, d, 2*time.Minute); err != nil {
					s.Error("Failed to ensure DUT is disconnected: ", err)
				}
			}
		})
	}
}
