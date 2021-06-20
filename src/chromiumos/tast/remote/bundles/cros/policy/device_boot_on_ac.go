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
	"chromiumos/tast/rpc"
	ps "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DeviceBootOnAc,
		Desc: "Tests the DeviceBootOnAcEnabled policy",
		Contacts: []string{
			"bisakhmondal00@gmail.com", // test author
			"lamzin@google.com",        // wilco_dtc_supportd maintainer
			"chromeos-wilco@google.com",
		},
		SoftwareDeps: []string{"wilco", "chrome"},
		Timeout:      25 * time.Minute,
		Attr:         []string{"group:enrollment"},
		ServiceDeps:  []string{"tast.cros.policy.PolicyService"},
		// For boot on AC, we are operating with two servos. One is "servo-power" to control
		// the charging state (power delivery [pd_role]) of DUT. The other one is the "servo-state",
		// the micro servo connected to the DUT motherboard that controls the on-off state of DUT.
		Vars: []string{"servo-power", "servo-state"},
	})
}

// DeviceBootOnAc verifies DeviceBootOnAcEnabled policy that boots the device from the off state by plugging
// in a power supply. If the policy is disabled or not set, boot on AC is off.
func DeviceBootOnAc(ctx context.Context, s *testing.State) {
	d := s.DUT()
	// isDischarging checks if the DUT is in discharging state.
	isDischarging := func(ctx context.Context) (bool, error) {
		out, err := d.Conn().Command("cat", "/sys/class/power_supply/BAT0/status").Output(ctx)
		if err != nil {
			return false, err
		}
		return strings.Contains(strings.TrimSpace(strings.ToLower(string(out))), "discharging"), nil
	}

	// Shorten deadline to leave time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Minute)
	defer cancel()
	defer func(ctx context.Context) {
		if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, d); err != nil {
			s.Error("Failed to reset TPM: ", err)
		}
	}(cleanupCtx)

	// Establishing proxy with two different servos.
	pxyState, err := servo.NewProxy(ctx, s.RequiredVar("servo-state"), d.KeyFile(), d.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxyState.Close(cleanupCtx)

	pxy, err := servo.NewProxy(ctx, s.RequiredVar("servo-power"), d.KeyFile(), d.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(cleanupCtx)

	// powerKeypress logically presses the power key in DUT by communicating with servod.
	powerKeypress := func(ctx context.Context) error {
		err := pxyState.Servo().KeypressWithDuration(ctx, servo.PowerKey, servo.DurLongPress)
		if err != nil {
			return errors.Wrap(err, "failed to press power key")
		}
		return nil
	}

	// ensureDUTisOn ensures the DUT is in powered on.
	ensureDUTisOn := func(ctx context.Context) error {
		if d.Connected(ctx) {
			return nil
		}
		// If DUT is not pingable, boot it up.
		if err := powerKeypress(ctx); err != nil {
			return err
		}

		s.Log("Reconnecting to DUT")
		if err := d.WaitConnect(ctx); err != nil {
			return errors.Wrap(err, "failed to connect to DUT")
		}
		s.Log("Reconnected to DUT")
		return nil
	}

	for _, tc := range []struct {
		state       string
		pol         policy.Policy
		wantRestart bool
	}{
		{
			state:       "not_set",
			pol:         &policy.DeviceBootOnAcEnabled{Stat: policy.StatusUnset},
			wantRestart: false,
		},
		{
			state:       "enabled",
			pol:         &policy.DeviceBootOnAcEnabled{Val: true},
			wantRestart: true,
		},
		{
			state:       "disabled",
			pol:         &policy.DeviceBootOnAcEnabled{Val: false},
			wantRestart: false,
		},
	} {
		s.Run(ctx, tc.state, func(ctx context.Context, s *testing.State) {
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
			pb.AddPolicy(tc.pol)

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
				s.Fatal("Unable to cut-off power supply: ", err)
			}

			// Ensuring DUT actually has started discharging. Using polling to tackle
			// the delay of the three power states in DUT i.e. Charging to Unknown to Discharging.
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				status, err := isDischarging(ctx)
				if err != nil {
					return testing.PollBreak(err)
				}
				if !status {
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
			if err := powerKeypress(ctx); err != nil {
				s.Fatal("Failed to power off DUT: ", err)
			}

			s.Log("Waiting for DUT to become unreachable")
			if err := d.WaitUnreachable(ctx); err != nil {
				s.Fatal("DUT is still reachable while it should not be: ", err)
			}
			s.Log("DUT became unreachable (as expected)")

			// Even after DUT becomes unreachable, it is not completely powered off.
			testing.Sleep(ctx, 15*time.Second)

			// Even if policy fails, device must be powered on.
			defer func(ctx context.Context) {
				if err := ensureDUTisOn(ctx); err != nil {
					s.Error("Failed to ensure DUT is powered on: ", err)
				}
			}(ctx)

			// Connecting DUT to the power supply to test the policy behaviour.
			if err := pxy.Servo().SetPDRole(ctx, servo.PDRoleSrc); err != nil {
				s.Fatal("Unable to turn on power supply: ", err)
			}

			waitCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			defer cancel()
			if err := d.WaitConnect(waitCtx); err != nil {
				if tc.wantRestart {
					s.Error("Failed subtest: device didn't boot up while it should be: ", err)
				}
				return
			}

			if !tc.wantRestart {
				s.Error("Failed subtest: device booted up while it should not be")
			}
		})
	}
}
