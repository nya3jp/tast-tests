// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

/*
This file implements miscellaneous and unsorted helpers.
*/

import (
	"context"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

type cmdRunnerRemote struct {
	d *dut.DUT
}

// Run implements the one of hwsec.CmdRunner.
func (r *cmdRunnerRemote) Run(ctx context.Context, cmd string, args ...string) ([]byte, error) {
	testing.ContextLogf(ctx, "Running: %s", shutil.EscapeSlice(append([]string{cmd}, args...)))
	return r.d.Command(cmd, args...).Output(ctx)
}

// RunShell implements the one of hwsec.CmdRunner.
func (r *cmdRunnerRemote) RunShell(ctx context.Context, cmd string) ([]byte, error) {
	return hwsec.RunShell(ctx, r, cmd)
}

// NewCmdRunner creates a new cmdRunnerRemote instance associated with |d|.
func NewCmdRunner(d *dut.DUT) (*cmdRunnerRemote, error) {
	return &cmdRunnerRemote{d}, nil
}

type helperRemote struct {
	hwsec.Helper
	ti hwsec.TpmInitializer
	d  *dut.DUT
}

// NewHelper creates a new hwsec.Helper instance that make use of the functions
// implemented by cmdRunnerRemote.
func NewHelper(ti hwsec.TpmInitializer, d *dut.DUT) (*helperRemote, error) {
	return &helperRemote{*hwsec.NewHelper(ti), ti, d}, nil
}

// EnsureTPMIsReset ensures the TPM is reset when the function returns |nil|.
// Otherwise, returns any encountered error.
func (h *helperRemote) EnsureTPMIsReset(ctx context.Context, r hwsec.CmdRunner) error {
	isReady, err := h.ti.IsTPMReady(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to check ownership due to error in |IsTPMReady|")
	}
	if !isReady {
		return nil
	}
	if _, err := r.RunShell(ctx, "crossystem clear_tpm_owner_request=1"); err != nil {
		return errors.Wrap(err, "failed to file clear_tpm_owner_request")
	}
	if err := h.Reboot(ctx); err != nil {
		return errors.Wrap(err, "failed to reboot")
	}
	isReady, err = h.ti.IsTPMReady(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to check if TPM is reset due to error in |IsTPMReady|")
	}
	if isReady {
		return errors.New("ineffective reset of tpm")
	}
	return nil
}

// Reboot reboots the DUT
func (h *helperRemote) Reboot(ctx context.Context) error {
	return h.d.Reboot(ctx)
}
