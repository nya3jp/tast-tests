// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/policyutil"
	"chromiumos/tast/remote/servo"
	"chromiumos/tast/rpc"
	ps "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DeviceBootOnAC,
		Desc: "DeviceBootOnAC verifies DeviceBootOnAcEnabled policy, it requires Wilco devices",
		Contacts: []string{
			"bisakhmondal00@gmail.com", // test author
			"lamzin@chromium.org",      // wilco_dtc_supportd maintainer
			"chromeos-wilco@google.com",
		},
		Attr:         []string{"group:enrollment"},
		SoftwareDeps: []string{"reboot", "wilco", "chrome"},
		Vars:         []string{"servo"},
		ServiceDeps:  []string{"tast.cros.policy.PolicyService"},
		HardwareDeps: hwdep.D(hwdep.Battery()),
		Timeout:      15 * time.Minute,
	})
}

// DeviceBootOnAC verifies DeviceBootOnACEnabled Policy.
// If this policy is set to true then boot on AC will always be enabled if supported on the device.
// If this policy is set to false, boot on AC will always be disabled.
// If this policy is left unset, boot on AC is disabled.
func DeviceBootOnAC(ctx context.Context, s *testing.State) {
	pxy, err := servo.NewProxy(ctx, s.RequiredVar("servo"), s.DUT().KeyFile(), s.DUT().KeyDir())
	if err != nil {
		s.Fatal("Failed to establish proxy with servo: ", err)
	}
	if isv4, err := pxy.Servo().IsServoV4(ctx); err != nil {
		s.Fatal("Test requires servo V4. failed to determine servo version: ", err)
	} else if !isv4 {
		s.Fatal("Unable to run the test, require servo V4 board")
	}

	checkDischarge := func(ctx context.Context, d *dut.DUT) (bool, error) {
		out, err := d.Conn().Command("cat", "/sys/class/power_supply/BAT0/status").Output(ctx)
		if err != nil {
			return false, err
		}
		return strings.Contains(strings.TrimSpace(strings.ToLower(string(out))), "discharging"), nil
	}

	ensureDUTIsON := func(ctx context.Context, d *dut.DUT, timeout time.Duration) error {
		reconctx, reconCancel := context.WithTimeout(ctx, timeout)
		defer reconCancel()

		if err := d.WaitConnect(reconctx); err != nil {
			return errors.Wrap(err, "failed to connect to DUT")
		}
		return nil
	}

	confirmDUTIsOFF := func(ctx context.Context, d *dut.DUT, timeout time.Duration) error {
		waitCtx, waitCancel := context.WithTimeout(ctx, timeout)
		defer waitCancel()
		if err := d.WaitUnreachable(waitCtx); err != nil {
			return errors.Wrap(err, "DUT is still reachable while it should not be")
		}
		return nil
	}

	var DUTisOnError = errors.New("DUT is ON")

	ensureDUTRemainsOFF := func(ctx context.Context, d *dut.DUT, timeout time.Duration) error {
		return testing.Poll(ctx, func(ctx context.Context) error {
			// If the DUT is powered OFF for that particular instance WaitUnreachable returns nil
			// Maybe by that time it's booting UP. We should wait for the whole duration.
			if err := confirmDUTIsOFF(ctx, d, timeout); err == nil {
				return DUTisOnError
			}
			return nil
		}, &testing.PollOptions{
			Timeout:  timeout,
			Interval: time.Second,
		})
	}

	pressPowerKey := func(ctx context.Context) error {
		return pxy.Servo().KeypressWithDuration(ctx, servo.PowerKey, servo.DurLongPress)
	}

	// Post test cleanup
	defer func(ctx context.Context) {
		// Revert role back to src
		if err := pxy.Servo().SetV4PDRole(ctx, servo.V4RoleSrc); err != nil {
			s.Error("Failed to set servo role to src")
		}

		// Power ON device if it is OFF
		if err := ensureDUTIsON(ctx, s.DUT(), 2*time.Second); err != nil {
			if err := pressPowerKey(ctx); err != nil {
				s.Fatal("Failed to press power key: ", err)
			}
			if err := ensureDUTIsON(ctx, s.DUT(), 60*time.Second); err != nil {
				s.Fatal("Failed to power on device: ", err)
			}
		}

		{
			cctx, ccancel := context.WithTimeout(ctx, 100*time.Second) // TODO: Remove. I am adding timeout to avoid long delay while re-establishing ssh to Oleh's device.
			defer ccancel()
			if err := policyutil.ClearTPMIfOwned(cctx, s.DUT()); err != nil {
				s.Error("Failed to clear the TPM: ", err)
			}
		}
	}(ctx)

	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Minute)
	defer cancel()

	for _, tc := range []struct {
		state string
		pol   *policy.DeviceBootOnAcEnabled
	}{
		{
			state: "enabled",
			pol:   &policy.DeviceBootOnAcEnabled{Val: true},
		},
		{
			state: "disabled",
			pol:   &policy.DeviceBootOnAcEnabled{Val: false},
		},
		{
			state: "not_set",
			pol:   &policy.DeviceBootOnAcEnabled{Stat: policy.StatusUnset},
		},
	} {
		s.Run(ctx, tc.state, func(ctx context.Context, s *testing.State) {
			{
				cctx, ccancel := context.WithTimeout(ctx, 100*time.Second) // TODO: Remove. I am adding timeout to avoid long delay while re-establishing ssh to Oleh's device.
				defer ccancel()
				if err := policyutil.ClearTPMIfOwned(cctx, s.DUT()); err != nil {
					s.Error("Failed to clear the TPM: ", err)
				}
			}
			// In case TPM powerwash succeeds but fails to re-establish connection
			if err := ensureDUTIsON(ctx, s.DUT(), 30*time.Second); err != nil {
				// Without DUT, testing fails by default
				s.Fatal("Error connecting DUT: ", err)
			}

			cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
			if err != nil {
				s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
			}
			defer cl.Close(ctx)

			polC := ps.NewPolicyServiceClient(cl.Conn)

			pb := fakedms.NewPolicyBlob()
			pb.AddPolicy(tc.pol)

			pJSON, err := json.Marshal(pb)
			if err != nil {
				s.Fatal("Error while marshalling policies to JSON: ", err)
			}

			if _, err := polC.EnrollUsingChrome(ctx, &ps.EnrollUsingChromeRequest{
				PolicyJson: pJSON,
			}); err != nil {
				s.Fatal("Failed to enroll policy using chrome: ", err)
			}

			// Putting servo into snk (sink mode) to simulate battery discharge.
			if err := pxy.Servo().SetV4PDRole(ctx, servo.V4RoleSnk); err != nil {
				s.Fatal("Unable to cut-off power supply: ", err)
			}

			// Wait till DUT is using battery.
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				disC, err := checkDischarge(ctx, s.DUT())
				if err != nil {
					return err
				}
				if !disC {
					return errors.New("device is not in discharging state")
				}
				return nil
			}, &testing.PollOptions{
				Timeout:  30 * time.Second,
				Interval: 2 * time.Second,
			}); err != nil {
				s.Fatal("Device is not using battery while it should: ", err)
			}

			s.Log("Powering Off DUT")
			if err := pressPowerKey(ctx); err != nil {
				s.Fatal("Failed to press power key: ", err)
			}

			if err := confirmDUTIsOFF(ctx, s.DUT(), 30*time.Second); err != nil {
				s.Fatal("Failed to power OFF DUT: ", err)
			}

			// Starting Power Supply
			if err := pxy.Servo().SetV4PDRole(ctx, servo.V4RoleSrc); err != nil {
				s.Error("Failed to provide power supply to the servo: ", err)
			}

			if tc.pol.Val {
				// Policy is enabled
				if err := ensureDUTIsON(ctx, s.DUT(), 50*time.Second); err != nil {
					s.Error("DUT is Powered OFF while it should be ON: ", err)
				}
			} else {
				if err := ensureDUTRemainsOFF(ctx, s.DUT(), 50*time.Second); err != nil {
					s.Error("Device should remain OFF but it's booted up: ", err)
					return
				}
				if err := pressPowerKey(ctx); err != nil {
					s.Fatal("Failed to press Power Key")
				}
				if err := ensureDUTIsON(ctx, s.DUT(), 60*time.Second); err != nil {
					s.Error("Failed to power ON the DUT")
				}
			}

		})
	}
}
