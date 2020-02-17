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
func (h *HelperRemote) EnsureTPMIsReset(ctx context.Context) error {
	isReady, err := h.ti.IsTPMReady(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to check ownership due to error in |IsTPMReady|")
	}
	if !isReady {
		return nil
	}
	if _, err := h.r.Run(ctx, "crossystem", "clear_tpm_owner_request=1"); err != nil {
		return errors.Wrap(err, "failed to file clear_tpm_owner_request")
	}
	if err := h.Reboot(ctx); err != nil {
		return errors.Wrap(err, "failed to reboot")
	}

	if err := WaitForSystemReady(ctx, h.ti); err != nil {
		return errors.Wrap(err, "TPM reset failed")
	}

	return nil
}

// WaitForSystemReady waits until cryptohome is ready and checks that the TPM has been reset
// after a reboot.
func WaitForSystemReady(ctx context.Context, utility hwsec.TPMInitializer) error {
	testing.ContextLog(ctx, "Waiting for system to be ready after reboot ")
	// Currently cryptohome is a bit slow on the first boot, tracked by crbug/879797, so this polling here is necessary to avoid flakiness. This polling can be removed if the referenced bug is fixed.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		_, err := utility.IsTPMReady(ctx)
		return err
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait for cryptohome")
	}

	isReady, err := utility.IsTPMReady(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to check whether TPM was reset")
	} else if isReady {
		// If the TPM is ready, the reset was not successful
		return errors.New("ineffective reset of tpm")
	}

	return nil
}

// Reboot reboots the DUT
func (h *HelperRemote) Reboot(ctx context.Context) error {
	return h.d.Reboot(ctx)
}

// EnsureOwnerIsReset removes ownership on the device.
// This allows it to be enrolled or to be used normally after enrollment.
func EnsureOwnerIsReset(ctx context.Context, d *dut.DUT) error {
	r, err := NewCmdRunner(d)
	if err != nil {
		return errors.Wrap(err, "CmdRunner creation error")
	}
	utility, err := hwsec.NewUtilityCryptohomeBinary(r)
	if err != nil {
		return errors.Wrap(err, "utility creation error")
	}

	isReady, err := utility.IsTPMReady(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to check whether TPM was ready")
	}
	if !isReady {
		return nil
	}

	if _, err := r.Run(ctx, "crossystem", "clear_tpm_owner_request=1"); err != nil {
		return errors.Wrap(err, "failed to file clear_tpm_owner_request")
	}

	d.Command("rm", "-rf",
		"/home/chronos/.oobe_completed",
		"/home/chronos/Local State",
		"/var/cache/shill/default.profile",
		"/home/.shadow/*",
		"/var/lib/whitelist/*",
		"/var/cache/app_pack",
		"/var/lib/tpm",
	).Run(ctx) // Ignore errors as files might have already been removed

	if err := d.Reboot(ctx); err != nil {
		return errors.Wrap(err, "failed to reboot")
	}

	if err := WaitForSystemReady(ctx, utility); err != nil {
		return errors.Wrap(err, "TPM reset failed")
	}

	testing.ContextLog(ctx, "TPM reset done")

	return nil
}
