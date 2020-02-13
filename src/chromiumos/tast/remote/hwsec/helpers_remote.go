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

// ensureTPMIsReset ensures the TPM is reset when the function returns nil.
// Otherwise, returns any encountered error.
// Optionally removes files from the DUT to simulate a powerwash.
func (h *HelperRemote) ensureTPMIsReset(ctx context.Context, removeFiles bool) error {
	// TODO(crbug.com/879797): Remove polling.
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

	if _, err := h.r.Run(ctx, "cryptohome", "--action=unmount"); err != nil {
		return errors.Wrap(err, "failed to unmount users")
	}

	if _, err := h.r.Run(ctx, "crossystem", "clear_tpm_owner_request=1"); err != nil {
		return errors.Wrap(err, "failed to file clear_tpm_owner_request")
	}

	if removeFiles {
		if out, err := h.d.Command("rm", "-rf", "--",
			"/home/chronos/.oobe_completed",
			"/home/chronos/Local State",
			"/var/cache/shill/default.profile",
			"/home/.shadow/",
			"/var/lib/whitelist/",
			"/var/cache/app_pack",
			"/var/lib/tpm",
		).CombinedOutput(ctx); err != nil {
			return errors.Wrapf(err, "failed to remove files to clear ownership: %s", string(out))
		}
	}

	if err := h.Reboot(ctx); err != nil {
		return errors.Wrap(err, "failed to reboot")
	}

	testing.ContextLog(ctx, "Waiting for system to be ready after reboot ")
	// TODO(crbug.com/879797): Remove polling.
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
	if err := h.d.Reboot(ctx); err != nil {
		return err
	}
	dCtrl := hwsec.NewDaemonController(h.r)
	// Waits for all the daemons of interest to be ready because the asynchronous initialization of dbus service could complete "after" the booting process.
	if err := dCtrl.WaitForAllDBusServices(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for hwsec D-Bus services to be ready")
	}
	return nil
}

// EnsureTPMIsReset ensures the TPM is reset when the function returns nil.
// Otherwise, returns any encountered error.
func (h *HelperRemote) EnsureTPMIsReset(ctx context.Context) error {
	return h.ensureTPMIsReset(ctx, false)
}

// EnsureTPMIsResetAndPowerwash ensures the TPM is reset and simulates a Powerwash.
func (h *HelperRemote) EnsureTPMIsResetAndPowerwash(ctx context.Context) error {
	return h.ensureTPMIsReset(ctx, true)
}
