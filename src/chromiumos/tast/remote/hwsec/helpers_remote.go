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

// CmdRunnerRemote implements CmdRunner for remote test.
type CmdRunnerRemote struct {
	d        *dut.DUT
	printLog bool
}

// Run implements the one of hwsec.CmdRunner.
func (r *CmdRunnerRemote) Run(ctx context.Context, cmd string, args ...string) ([]byte, error) {
	if r.printLog {
		testing.ContextLogf(ctx, "Running: %s", shutil.EscapeSlice(append([]string{cmd}, args...)))
	}
	return r.d.Command(cmd, args...).Output(ctx)
}

// NewCmdRunner creates a new CmdRunnerRemote instance associated with d.
func NewCmdRunner(d *dut.DUT) (*CmdRunnerRemote, error) {
	return &CmdRunnerRemote{d: d, printLog: true}, nil
}

// NewLoglessCmdRunner creates a new CmdRunnerRemote instance associated with d, which wouldn't print logs.
func NewLoglessCmdRunner(d *dut.DUT) (*CmdRunnerRemote, error) {
	return &CmdRunnerRemote{d: d, printLog: false}, nil
}

// HelperRemote extends the function set from hwsec.Helper for remote test.
type HelperRemote struct {
	hwsec.Helper
	d *dut.DUT
}

// NewHelper creates a new hwsec.Helper instance that make use of the functions
// implemented by CmdRunnerRemote.
func NewHelper(r hwsec.CmdRunner, d *dut.DUT) (*HelperRemote, error) {
	tpmClearer := NewTPMClearer(r, d)
	helper, err := hwsec.NewHelper(r, tpmClearer)
	if err != nil {
		return nil, err
	}
	return &HelperRemote{*helper, d}, nil
}

// Reboot reboots the DUT
func (h *HelperRemote) Reboot(ctx context.Context) error {
	if err := h.d.Reboot(ctx); err != nil {
		return err
	}
	dCtrl := h.DaemonController()
	// Waits for all the daemons of interest to be ready because the asynchronous initialization of dbus service could complete "after" the booting process.
	if err := dCtrl.WaitForAllDBusServices(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for hwsec D-Bus services to be ready")
	}
	return nil
}
