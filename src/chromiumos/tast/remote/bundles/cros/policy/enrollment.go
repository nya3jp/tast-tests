// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"strings"
	"chromiumos/tast/dut"

	"chromiumos/tast/testing"
)

// ClearTPMIfOwned will clear the DUT's TPM, if the either the "TPMOwned" or
// "TPMBeingOwned" is true. Be default will also reboot the device after the clear.
// If status still shows as owned after the clear, an err is reaised.
func ClearTPMIfOwned(ctx context.Context, s *testing.State, doReboot bool) (error) {
    d, ok := dut.FromContext(ctx)
    if !ok {
        s.Fatal("Failed to get DUT")
    }

    TPMOwned := isTPMOwned(ctx, s)

	if TPMOwned {
		testing.ContextLog(ctx, "TPM owned: ", (TPMOwned))
		if err := d.Command("stop", "ui").Run(ctx); err != nil {
			s.Fatal("Failed to stop UI ", err)
		}

		if err := d.Command("crossystem", "clear_tpm_owner_request=1").Run(ctx); err != nil {
			s.Fatal("Failed to run crossystem clear_tpm_owner_request ", err)
		}

		if err := d.Command("rm", "-rf", "/home/.shadow/*", "/var/lib/whitelist/*", "/home/chronos/Local", "State").Run(ctx); err != nil {
			s.Fatal("Failed to rmrf DUT: ", err)
		}

		if doReboot {
			reboot(ctx, s)
			TPMOwnedPostReset := isTPMOwned(ctx, s)
			if TPMOwnedPostReset {
				s.Fatal("Unable to Clear TPM")
			}
		}


	}
	return nil
}

func isTPMOwned(ctx context.Context, s *testing.State) (bool){
    d, ok := dut.FromContext(ctx)
    if !ok {
        s.Fatal("Failed to get DUT")
    }
	out, err := d.Command("cryptohome", "--action=tpm_status").CombinedOutput(ctx)
	tpm := false
	if err != nil {
		s.Fatal("Failed to runcmd: ", err)
		}
	for _, line := range strings.Split(string(out), "\n") {
		substr := strings.Split(string(strings.Replace(line, " ", "", -1)), ":")
		if len(substr) != 2 {
			continue
		}

		switch {
			case substr[0] == "TPMOwned" && substr[1] == "true":
				tpm = true
			case substr[0] == "TPMBeingOwned" && substr[1] == "true":
				tpm = true
		}

	}
	return tpm
}

// TODO remove this once the global reboot in power works.
func reboot(ctx context.Context, s *testing.State) {
    d, ok := dut.FromContext(ctx)
    if !ok {
        s.Fatal("Failed to get DUT")
    }

    s.Log("Rebooting DUT")
    // Run the reboot command in the background to avoid the DUT potentially going down before
    // success is reported over the SSH connection. Redirect all I/O streams to ensure that the
    // SSH exec request doesn't hang (see https://en.wikipedia.org/wiki/Nohup#Overcoming_hanging).
    cmd := "nohup sh -c 'sleep 2; reboot' >/dev/null 2>&1 </dev/null &"
    if err := d.Command("sh", "-c", cmd).Run(ctx); err != nil {
        s.Fatal("Failed to reboot DUT: ", err)
    }

    s.Log("Waiting for DUT to become unreachable")
    if err := d.WaitUnreachable(ctx); err != nil {
        s.Fatal("Failed to wait for DUT to become unreachable: ", err)
    }
    s.Log("DUT became unreachable (as expected)")

    s.Log("Reconnecting to DUT")
    if err := d.WaitConnect(ctx); err != nil {
        s.Fatal("Failed to reconnect to DUT: ", err)
    }
    s.Log("Reconnected to DUT")
}
