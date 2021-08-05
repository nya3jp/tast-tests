// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"encoding/json"
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
		Func: DeviceUSBPowershare,
		Desc: "Tests the DeviceUsbPowerShareEnabled policy that shares power through USB when the device is off",
		Contacts: []string{
			"bisakhmondal00@gmail.com", // Test author
			"lamzin@google.com",        // wilco_dtc_supportd maintainer
			"chromeos-wilco@google.com",
		},
		Attr:         []string{"group:enrollment"},
		SoftwareDeps: []string{"chrome", "wilco"},
		ServiceDeps:  []string{"tast.cros.policy.PolicyService"},
		Timeout:      20 * time.Minute,
		Vars:         []string{"servo"},
	})
}

// DeviceUSBPowershare verifies DeviceUsbPowerShareEnabled policy that enables sharing power through USB
// when DUT is in a power-off state. If the policy is disabled, no power through USB and if it is unset,
// it acts as enabled.
func DeviceUSBPowershare(ctx context.Context, s *testing.State) {
	d := s.DUT()

	// Shorten deadline to leave time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Minute)
	defer cancel()
	defer func(ctx context.Context) {
		if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, d); err != nil {
			s.Error("Failed to reset TPM: ", err)
		}
	}(cleanupCtx)

	pxy, err := servo.NewProxy(ctx, s.RequiredVar("servo"), d.KeyFile(), d.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(cleanupCtx)

	// averagePowerOutput returns the average power output through the DUT USB VBUS interface.
	// Servo vbus_power sometimes returns 0.0 (when the connected device isn't drawing power for an instant)
	// that's why instead of depending on single power output, we are taking the average.
	averagePowerOutput := func(ctx context.Context) (float64, error) {
		times, sum := 7, 0.0
		for i := 0; i < times; i++ {
			p, err := pxy.Servo().GetFloat(ctx, servo.FloatControl("vbus_power"))
			if err != nil {
				return 0.0, errors.Wrap(err, "failed to receive vbus_power output through servo")
			}
			sum += p
			if err := testing.Sleep(ctx, time.Second); err != nil {
				return 0.0, errors.Wrap(err, "failed to sleep")
			}
		}
		return sum / float64(times), nil
	}

	for _, tc := range []struct {
		state          string
		pol            policy.Policy
		wantPowerShare bool
	}{
		{
			state:          "unset",
			pol:            &policy.DeviceUsbPowerShareEnabled{Stat: policy.StatusUnset},
			wantPowerShare: true,
		},
		{
			state:          "enabled",
			pol:            &policy.DeviceUsbPowerShareEnabled{Val: true},
			wantPowerShare: true,
		},
		{
			state:          "disabled",
			pol:            &policy.DeviceUsbPowerShareEnabled{Val: false},
			wantPowerShare: false,
		},
	} {
		s.Run(ctx, tc.state, func(ctx context.Context, s *testing.State) {
			if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, d); err != nil {
				s.Error("Failed to clear TPM: ", err)
			}

			cl, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
			if err != nil {
				s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
			}

			pc := ps.NewPolicyServiceClient(cl.Conn)
			pb := fakedms.NewPolicyBlob()
			pb.AddPolicy(tc.pol)

			pJSON, err := json.Marshal(pb)
			if err != nil {
				s.Fatal("Error while marshalling policies to JSON: ", err)
			}

			if _, err := pc.EnrollUsingChrome(ctx, &ps.EnrollUsingChromeRequest{
				PolicyJson: pJSON,
			}); err != nil {
				s.Fatal("Failed to enroll using chrome: ", err)
			}

			// Powering off DUT and wait until it is unreachable.
			if err := pxy.Servo().KeypressWithDuration(ctx, servo.PowerKey,
				servo.DurLongPress); err != nil {
				s.Fatal("Failed to power off DUT: ", err)
			}

			s.Log("Waiting for DUT to become unreachable")
			if err := d.WaitUnreachable(ctx); err != nil {
				s.Fatal("Failed to power off the device: ", err)
			}
			s.Log("DUT became unreachable as expected")

			// Even after DUT becomes unreachable, it is not completely powered off.
			if err := testing.Sleep(ctx, 15*time.Second); err != nil {
				s.Error("Failed to sleep: ", err)
			}
			// Checking VBUS power output.
			receivedPower, err := averagePowerOutput(ctx)
			if err != nil {
				s.Fatal("Failed to receive power output: ", err)
			}
			if tc.wantPowerShare && receivedPower == 0.0 {
				s.Error("DUT is not sharing power while it should. Power received: ", receivedPower)
			}
			if !tc.wantPowerShare && receivedPower != 0.0 {
				s.Error("DUT is sharing power while it should not. Power received: ", receivedPower)
			}

			// Powering on DUT and wait till it becomes reachable.
			if err := pxy.Servo().KeypressWithDuration(ctx, servo.PowerKey,
				servo.DurLongPress); err != nil {
				s.Fatal("Failed to power on DUT: ", err)
			}

			s.Log("Reconnecting to DUT")
			if err := d.WaitConnect(ctx); err != nil {
				s.Fatal("Failed to reconnect to DUT: ", err)
			}
			s.Log("Reconnected to DUT")
		})
	}
}
