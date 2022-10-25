// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"encoding/json"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/policy/dututils"
	"chromiumos/tast/remote/policyutil"
	"chromiumos/tast/rpc"
	pspb "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DeviceUSBPowershare,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests the DeviceUsbPowerShareEnabled policy that shares power through USB when the device is off",
		Contacts: []string{
			"chromeos-oem-services@google.com", // Use team email for tickets.
			"bkersting@google.com",
			"lamzin@google.com",
		},
		Attr:         []string{"group:wilco_bve"},
		SoftwareDeps: []string{"chrome", "wilco"},
		ServiceDeps: []string{
			"tast.cros.hwsec.OwnershipService",
			"tast.cros.policy.PolicyService",
		},
		Timeout: 20 * time.Minute,
		// Var "servo" is a ServoV4 Type-A device paired with a Servo Micro via the micro USB port.
		// Servo Micro as usual gets connected to the DUT motherboard debug header and the other cable
		// with a USB-A head is attached to the DUT type A port having a lightning bolt or a battery icon.
		// Note: both cables must be connected to the DUT.
		Vars: []string{"servo"},
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

	pxy, err := servo.NewProxy(ctx, s.RequiredVar("servo"), d.KeyFile(), d.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(cleanupCtx)

	defer func(ctx context.Context) {
		if err := dututils.EnsureDUTIsOn(ctx, d, pxy.Servo()); err != nil {
			s.Error("Failed to ensure DUT is powered on: ", err)
		}
		if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, d, s.RPCHint()); err != nil {
			s.Error("Failed to reset TPM after test: ", err)
		}
	}(cleanupCtx)

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
		name           string
		policy         policy.Policy
		wantPowerShare bool
	}{
		{
			name:           "unset",
			policy:         &policy.DeviceUsbPowerShareEnabled{Stat: policy.StatusUnset},
			wantPowerShare: true,
		},
		{
			name:           "enabled",
			policy:         &policy.DeviceUsbPowerShareEnabled{Val: true},
			wantPowerShare: true,
		},
		{
			name:           "disabled",
			policy:         &policy.DeviceUsbPowerShareEnabled{Val: false},
			wantPowerShare: false,
		},
	} {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			// For safety purpose, introducing a new cleanup context for device boot up.
			cleanupCtx := ctx
			ctx, cancel := ctxutil.Shorten(ctx, 2*time.Minute)
			defer cancel()

			if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, d, s.RPCHint()); err != nil {
				s.Error("Failed to clear TPM: ", err)
			}

			cl, err := rpc.Dial(ctx, d, s.RPCHint())
			if err != nil {
				s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
			}

			policyClient := pspb.NewPolicyServiceClient(cl.Conn)
			pb := policy.NewBlob()
			pb.AddPolicy(tc.policy)

			pJSON, err := json.Marshal(pb)
			if err != nil {
				s.Fatal("Error while marshalling policies to JSON: ", err)
			}

			if _, err := policyClient.EnrollUsingChrome(ctx, &pspb.EnrollUsingChromeRequest{
				PolicyJson: pJSON,
			}); err != nil {
				s.Fatal("Failed to enroll using chrome: ", err)
			}

			// Even if policy fails, device must be on a power on state in between subtests.
			defer func(ctx context.Context) {
				if err := dututils.EnsureDUTIsOn(ctx, d, pxy.Servo()); err != nil {
					s.Error("Failed to ensure DUT is powered on: ", err)
				}
			}(cleanupCtx)

			// Powering off DUT and wait until it is unreachable.
			if err := pxy.Servo().KeypressWithDuration(ctx, servo.PowerKey, servo.DurLongPress); err != nil {
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
				s.Error("DUT is not sharing power while it should")
			}
			if !tc.wantPowerShare && receivedPower != 0.0 {
				s.Errorf("DUT is sharing power while it should not. Power received: %.2f", receivedPower)
			}
		})
	}
}
