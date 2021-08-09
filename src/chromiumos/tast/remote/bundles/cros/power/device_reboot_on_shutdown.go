// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"encoding/json"
	"path/filepath"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/policyutil"
	"chromiumos/tast/rpc"
	ps "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DeviceRebootOnShutdown,
		Desc: "Tests the DeviceRebootOnShutdown policy that performs automatic reboot on device shutdown",
		Contacts: []string{
			"bisakhmondal00@gmail.com", // test author
			"lamzin@google.com",        // wilco_dtc_supportd maintainer
			"chromeos-wilco@google.com",
		},
		SoftwareDeps: []string{"wilco", "chrome"},
		Timeout:      25 * time.Minute,
		Attr:         []string{"group:enrollment"},
		ServiceDeps:  []string{"tast.cros.policy.PolicyService"},
		Vars:         []string{"servo"},
	})
}

// DeviceRebootOnShutdown tests the DeviceRebootOnShutdown policy which if enabled replaces all shutdown buttons
// in the ui with restart buttons. This does not hold true if the device has been shut down using the power button,
// even if the policy is enabled.
// If the policy is disabled or not set, the device just shuts down and no reboot is performed.
func DeviceRebootOnShutdown(ctx context.Context, s *testing.State) {
	// In case of ui error, dumpDir stores the ui dump tree inside DUT.
	const dumpDir = "/tmp/ui_dump_device_reboot_on_shutdown"
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

	// ensureDUTisOn ensures the DUT is in power-on state.
	ensureDUTisOn := func(ctx context.Context) error {
		// Try connecting with a small context, if DUT is already up, it will be successful.
		waitCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if err := d.WaitConnect(waitCtx); err == nil {
			return nil
		}
		// If DUT is not reachable, boot it up.
		if err := pxy.Servo().KeypressWithDuration(ctx, servo.PowerKey, servo.DurLongPress); err != nil {
			return errors.Wrap(err, "failed to press power key")
		}

		s.Log("Reconnecting to DUT")
		if err := d.WaitConnect(ctx); err != nil {
			return errors.Wrap(err, "failed to connect to DUT")
		}
		s.Log("Reconnected to DUT")
		return nil
	}

	// collectUIDump fetches faillog (ui dump tree and screenshot) from DUT and make it available at test results.
	collectUIDump := func(ctx context.Context) error {
		s.Log("Collecting ui tree dump and screenshot (faillog) from DUT")
		if err := linuxssh.GetFile(ctx, d.Conn(), dumpDir, filepath.Join(s.OutDir(), "ui_dump"), linuxssh.PreserveSymlinks); err != nil {
			return errors.Wrap(err, "failed to pull faillog files from DUT")
		}
		if err := d.Command("rm", "-rf", dumpDir).Run(ctx); err != nil {
			return errors.Wrap(err, "failed to clean up faillog files from DUT")
		}
		s.Log("Faillog collected successfully")
		return nil
	}

	for _, tc := range []struct {
		state       string
		pol         policy.Policy
		wantRestart bool
		// Upon successful policy registration (enable), chromeos renames node "Shut down" to "Restart".
		uiNodeName string
	}{
		{
			state:       "not_set",
			pol:         &policy.DeviceRebootOnShutdown{Stat: policy.StatusUnset},
			wantRestart: false,
			uiNodeName:  "Shut down",
		},
		{
			state:       "enabled",
			pol:         &policy.DeviceRebootOnShutdown{Val: true},
			wantRestart: true,
			uiNodeName:  "Restart",
		},
		{
			state:       "disabled",
			pol:         &policy.DeviceRebootOnShutdown{Val: false},
			wantRestart: false,
			uiNodeName:  "Shut down",
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

			// Clicking shutdown button from chromebook ui system tray.
			if _, err := pc.PerformUIShutdown(ctx, &ps.PerformUIShutdownRequest{
				NodeName:  tc.uiNodeName,
				UiDumpDir: dumpDir,
			}); err != nil {
				if err := collectUIDump(ctx); err != nil {
					s.Error("Failed to collect ui dump tree: ", err)
				}
				s.Fatal("Failed to perform shutdown through ui: ", err)
			}

			s.Log("Waiting for DUT to become unreachable")
			if err := d.WaitUnreachable(ctx); err != nil {
				s.Fatal("DUT is still reachable while it should not be: ", err)
			}
			s.Log("DUT became unreachable (as expected)")

			// Even if policy fails, device must be powered on.
			defer func(ctx context.Context) {
				if err := ensureDUTisOn(ctx); err != nil {
					s.Error("Failed to ensure DUT is powered on: ", err)
				}
			}(ctx)

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
