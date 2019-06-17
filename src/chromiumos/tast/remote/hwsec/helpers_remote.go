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

// NewCmdRunnerRemote creates a new cmdRunnerRemote instance associated with |d|.
func NewCmdRunnerRemote(d *dut.DUT) (*cmdRunnerRemote, error) {
	return &cmdRunnerRemote{d}, nil
}

// HelperRemote subclasses common/hwsec.Helper and implements the helper functions for remote tests.
type HelperRemote struct {
	hwsec.Helper
	d *dut.DUT
}

// NewHelperRemote creates a new hwsec.Helper instance that make use of the functions
// implemented by cmdRunnerRemote.
func NewHelperRemote(d *dut.DUT) (*HelperRemote, error) {
	runner, err := NewCmdRunnerRemote(d)
	if err != nil {
		return nil, errors.Wrap(err, "error creating command runner")
	}
	return &HelperRemote{hwsec.Helper{runner}, d}, nil
}

// EnsureTPMIsReset ensures the TPM is reset when the function returns |nil|.
// Otherwise, returns any encountered error.
func (h *HelperRemote) EnsureTPMIsReset(ctx context.Context, utility hwsec.Utility) error {
	isReady, err := utility.IsTPMReady()
	if err != nil {
		return errors.Wrap(err, "failed to check ownership due to error in |IsTPMReady|")
	}
	if !isReady {
		return nil
	}
	if _, err := h.RunShell(ctx, "crossystem clear_tpm_owner_request=1"); err != nil {
		return errors.Wrap(err, "failed to file clear_tpm_owner_request")
	}
	if err := h.Reboot(ctx); err != nil {
		return errors.Wrap(err, "failed to reboot")
	}
	isReady, err = utility.IsTPMReady()
	if err != nil {
		return errors.Wrap(err, "failed to check if TPM is reset due to error in |IsTPMReady|")
	}
	if isReady {
		return errors.New("ineffective reset of tpm")
	}
	return nil
}

// Reboot reboots the DUT
func (h *HelperRemote) Reboot(ctx context.Context) error {
	return h.d.Reboot(ctx)
}
