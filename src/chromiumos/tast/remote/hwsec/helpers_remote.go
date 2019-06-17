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

// CmdRunnerRemote implements hwsec.CmdRunner in remote test.
type CmdRunnerRemote struct {
	d *dut.DUT
}

// Run implements the one of hwsec.CmdRunner.
func (r *CmdRunnerRemote) Run(ctx context.Context, cmd string, args ...string) ([]byte, error) {
	testing.ContextLogf(ctx, "Running: %s", shutil.EscapeSlice(append([]string{cmd}, args...)))
	return r.d.Command(cmd, args...).Output(ctx)
}

// NewCmdRunnerRemote creates a new CmdRunnerRemote instance associated with |d|.
func NewCmdRunnerRemote(d *dut.DUT) (*CmdRunnerRemote, error) {
	return &CmdRunnerRemote{d}, nil
}

// DUTRebooterRemote implements hwsec.DUTRebooter in remote test.
type DUTRebooterRemote struct {
	d *dut.DUT
}

// Reboot implements the one of hwsec.DUTRebooter.
func (r *DUTRebooterRemote) Reboot(ctx context.Context) error {
	return r.d.Reboot(ctx)
}

// NewDUTRebooterRemote creates a new DUTRebooterRemote instance associated with |d|.
func NewDUTRebooterRemote(d *dut.DUT) (*DUTRebooterRemote, error) {
	if d == nil {
		return nil, errors.New("bad DUT instance")
	}
	return &DUTRebooterRemote{d}, nil
}

// NewHelperRemote creates a new hwsec.Helper instance that make use of the functions
// implemented by CmdRunnerRemote and NewDUTRebooterRemote
func NewHelperRemote(d *dut.DUT) (*hwsec.Helper, error) {
	runner, err := NewCmdRunnerRemote(d)
	if err != nil {
		return nil, errors.Wrap(err, "error creating command runner")
	}
	rebooter, err := NewDUTRebooterRemote(d)
	if err != nil {
		return nil, errors.Wrap(err, "error creating rebooter")
	}
	return &hwsec.Helper{runner, rebooter}, nil
}
