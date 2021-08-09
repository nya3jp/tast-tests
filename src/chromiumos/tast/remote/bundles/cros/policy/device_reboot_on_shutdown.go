// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

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
	"chromiumos/tast/remote/bundles/cros/policy/dututils"
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
		SoftwareDeps: []string{"chrome"},
		Timeout:      30 * time.Minute,
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

	pxy, err := servo.NewProxy(ctx, s.RequiredVar("servo"), d.KeyFile(), d.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(cleanupCtx)

	defer func(ctx context.Context) {
		if err := dututils.EnsureDUTIsOn(ctx, d, pxy.Servo()); err != nil {
			s.Error("Failed to ensure DUT is powered on: ", err)
		}
		if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, d); err != nil {
			s.Error("Failed to reset TPM: ", err)
		}
	}(cleanupCtx)

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
		name        string
		policy      policy.Policy
		wantRestart bool
		// Upon successful policy registration (enable), chromeos renames node "Shut down" to "Restart".
		uiNodeName string
	}{
		{
			name:        "unset",
			policy:      &policy.DeviceRebootOnShutdown{Stat: policy.StatusUnset},
			wantRestart: false,
			uiNodeName:  "Shut down",
		},
		{
			name:        "enabled",
			policy:      &policy.DeviceRebootOnShutdown{Val: true},
			wantRestart: true,
			uiNodeName:  "Restart",
		},
		{
			name:        "disabled",
			policy:      &policy.DeviceRebootOnShutdown{Val: false},
			wantRestart: false,
			uiNodeName:  "Shut down",
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

			// Even if policy fails, device must be powered on.
			defer func(ctx context.Context) {
				if err := dututils.EnsureDUTIsOn(ctx, d, pxy.Servo()); err != nil {
					s.Error("Failed to ensure DUT is powered on: ", err)
				}
			}(cleanupCtx)

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
			s.Log("DUT became unreachable as expected")

			if tc.wantRestart {
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
