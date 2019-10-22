// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"strings"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// ClearTPMIfOwned will clear the DUT's TPM, if the either the "TPMOwned" or
// "TPMBeingOwned" is true. Be default will also reboot the device after the clear.
// If status still shows as owned after the clear, an err is reaised.
func ClearTPMIfOwned(ctx context.Context, doReboot bool) error {
	d, ok := dut.FromContext(ctx)
	if !ok {
		return errors.New("failed to get DUT from ctx")
	}

	TPMOwned, err := isTPMOwned(ctx)
	if err != nil {
		return err
	}
	if TPMOwned {
		testing.ContextLog(ctx, "TPM owned: ", (TPMOwned))
		if err := d.Command("stop", "ui").Run(ctx); err != nil {
			return err
		}

		if err := d.Command("crossystem", "clear_tpm_owner_request=1").Run(ctx); err != nil {
			return err
		}

		if err := d.Command("sh", "-c", "rm -rf /home/chronos/Local\\ State /var/lib/whitelist/* /home/.shadow/*").Run(ctx); err != nil {
			return err
		}

		if doReboot {
			if err := reboot(ctx); err != nil {
				return err
			}

			TPMOwnedPostReset, err := isTPMOwned(ctx)
			if err != nil {
				return err
			}

			if TPMOwnedPostReset {
				return errors.New("unable to clear TPM")
			}
		}
	}
	return nil
}

func isTPMOwned(ctx context.Context) (bool, error) {
	d, ok := dut.FromContext(ctx)
	if !ok {
		return false, errors.New("failed to get DUT from ctx")
	}

	out, err := d.Command("cryptohome", "--action=tpm_status").Output(ctx)
	if err != nil {
		return false, err
	}

	tpm := false
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
	return tpm, nil
}

// TODO remove this once the global reboot in power works.
func reboot(ctx context.Context) error {
	d, ok := dut.FromContext(ctx)
	if !ok {
		return errors.New("failed to get DUT from ctx")
	}

	// Run the reboot command in the background to avoid the DUT potentially going down before
	// success is reported over the SSH connection. Redirect all I/O streams to ensure that the
	// SSH exec request doesn't hang (see https://en.wikipedia.org/wiki/Nohup#Overcoming_hanging).
	cmd := "nohup sh -c 'sleep 2; reboot' >/dev/null 2>&1 </dev/null &"
	if err := d.Command("sh", "-c", cmd).Run(ctx); err != nil {
		return errors.Wrap(err, "failed to reboot dut")
	}

	if err := d.WaitUnreachable(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for DUT to become unreachable")
	}

	if err := d.WaitConnect(ctx); err != nil {
		return errors.Wrap(err, "failed to reconnect to DUT")
	}
	return nil
}
