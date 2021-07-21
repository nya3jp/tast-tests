// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

/*
This file implements command runner for remote tests.
*/

import (
	"context"

	"golang.org/x/crypto/ssh"

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

// NewCmdRunner creates a new CmdRunnerRemote instance associated with d.
func NewCmdRunner(d *dut.DUT) *CmdRunnerRemote {
	return &CmdRunnerRemote{d: d, printLog: true}
}

// NewLoglessCmdRunner creates a new CmdRunnerRemote instance associated with d, which wouldn't print logs.
func NewLoglessCmdRunner(d *dut.DUT) *CmdRunnerRemote {
	return &CmdRunnerRemote{d: d, printLog: false}
}

// Run implements hwsec.CmdRunner.Run.
func (r *CmdRunnerRemote) Run(ctx context.Context, cmd string, args ...string) ([]byte, error) {
	if r.printLog {
		testing.ContextLogf(ctx, "Running: %s", shutil.EscapeSlice(append([]string{cmd}, args...)))
	}
	result, err := r.d.Conn().CommandContext(ctx, cmd, args...).Output()
	err = checkExitError(cmd, err)
	return result, err
}

// RunWithCombinedOutput implements hwsec.CmdRunner.RunWithCombinedOutput.
func (r *CmdRunnerRemote) RunWithCombinedOutput(ctx context.Context, cmd string, args ...string) ([]byte, error) {
	if r.printLog {
		testing.ContextLogf(ctx, "Running: %s", shutil.EscapeSlice(append([]string{cmd}, args...)))
	}
	result, err := r.d.Conn().CommandContext(ctx, cmd, args...).CombinedOutput()
	err = checkExitError(cmd, err)
	return result, err
}

func checkExitError(cmd string, err error) error {
	if e, ok := err.(*ssh.ExitError); ok {
		err = &hwsec.CmdExitError{
			E:        errors.Wrapf(err, "failed %q command with error code %d", cmd, e.ExitStatus()),
			ExitCode: e.ExitStatus(),
		}
	}
	return err
}
