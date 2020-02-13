// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

/*
This file implements miscellaneous and unsorted helpers.
*/

import (
	"context"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

// CmdRunnerRemote implements CmdRunner for remote test.
type CmdRunnerRemote struct {
	d *dut.DUT
}

// Run implements the one of hwsec.CmdRunner.
func (r *CmdRunnerRemote) Run(ctx context.Context, cmd string, args ...string) ([]byte, error) {
	testing.ContextLogf(ctx, "Running: %s", shutil.EscapeSlice(append([]string{cmd}, args...)))
	return r.d.Command(cmd, args...).Output(ctx)
}

// NewCmdRunner creates a new CmdRunnerRemote instance associated with |d|.
func NewCmdRunner(d *dut.DUT) (*CmdRunnerRemote, error) {
	return &CmdRunnerRemote{d}, nil
}

// HelperRemote extends the function set from hwsec.Helper for remote test.
type HelperRemote struct {
	hwsec.Helper
	ti hwsec.TPMInitializer
	r  hwsec.CmdRunner
	d  *dut.DUT
}

// NewHelper creates a new hwsec.Helper instance that make use of the functions
// implemented by CmdRunnerRemote.
func NewHelper(ti hwsec.TPMInitializer, r hwsec.CmdRunner, d *dut.DUT) (*HelperRemote, error) {
	return &HelperRemote{*hwsec.NewHelper(ti), ti, r, d}, nil
}

// EnsureTPMIsReset ensures the TPM is reset when the function returns |nil|.
// Otherwise, returns any encountered error.
// Optionally removes files from the DUT to simulate a powerwash.
func (h *HelperRemote) EnsureTPMIsReset(ctx context.Context, removeFiles bool) error {
	// TODO(crbug/879797): Remove polling.
	// Currently cryptohome is a bit slow on the first boot, so this polling here is necessary to avoid flakiness.
	isReady := false
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		isReady, err = h.ti.IsTPMReady(ctx)
		return err
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait for cryptohome")
	}

	if !isReady {
		// TPM has already been reset by a previous test so we can skip doing it.
		return nil
	}

	if _, err := h.r.Run(ctx, "crossystem", "clear_tpm_owner_request=1"); err != nil {
		return errors.Wrap(err, "failed to file clear_tpm_owner_request")
	}

	if removeFiles {
		h.d.Command("rm", "-rf",
			"/home/chronos/.oobe_completed",
			"/home/chronos/Local State",
			"/var/cache/shill/default.profile",
			"/home/.shadow/*",
			"/var/lib/whitelist/*",
			"/var/cache/app_pack",
			"/var/lib/tpm",
		).Run(ctx) // Ignore errors as files might have already been removed
	}

	if err := h.Reboot(ctx); err != nil {
		return errors.Wrap(err, "failed to reboot")
	}

	testing.ContextLog(ctx, "Waiting for system to be ready after reboot ")
	// TODO(crbug/879797): Remove polling.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		_, err := h.ti.IsTPMReady(ctx)
		return err
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait for cryptohome")
	}

	isReady, err := h.ti.IsTPMReady(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to check whether TPM was reset")
	} else if isReady {
		// If the TPM is ready, the reset was not successful
		return errors.New("ineffective reset of TPM")
	}

	return nil
}

// Reboot reboots the DUT
func (h *HelperRemote) Reboot(ctx context.Context) error {
	return h.d.Reboot(ctx)
}

// EnsureTPMIsReset initialises the required helpers and calls HelperRemote.EnsureTPMIsReset.
func EnsureTPMIsReset(ctx context.Context, d *dut.DUT, removeFiles bool) error {
	r, err := NewCmdRunner(d)
	if err != nil {
		return errors.Wrap(err, "CmdRunner creation error")
	}
	utility, err := hwsec.NewUtilityCryptohomeBinary(r)
	if err != nil {
		return errors.Wrap(err, "utility creation error")
	}

	helper, err := NewHelper(utility, r, d)
	if err != nil {
		return errors.Wrap(err, "helper creation error")
	}

	if err := helper.EnsureTPMIsReset(ctx, removeFiles); err != nil {
		return errors.Wrap(err, "failed to reset system")
	}

	return nil
}
