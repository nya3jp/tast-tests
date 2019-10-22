// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// ClearTPMIfOwned will clear the DUT's TPM, if the either the "TPMOwned" or
// "TPMBeingOwned" is true. Be default will also reboot the device after the clear.
// If status still shows as owned after the clear, an err is reaised.
func ClearTPMIfOwned(ctx context.Context, doReboot bool, s *testing.State) error {
	d := s.DUT()

	TPMOwned, err := isTPMOwned(ctx, s)
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
			if err := d.Reboot(ctx); err != nil {
				return err
			}

			TPMOwnedPostReset, err := isTPMOwned(ctx, s)
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

func isTPMOwned(ctx context.Context, s *testing.State) (bool, error) {
	d := s.DUT()

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
