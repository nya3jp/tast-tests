// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

/*
This file implements miscellaneous and unsorted helpers.
*/

import (
	"context"
	"os/exec"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

// CmdRunnerLocal implements CmdRunner for local test.
type CmdRunnerLocal struct {
	printLog bool
}

// NewCmdRunner creates a new command runner for local test.
func NewCmdRunner() (*CmdRunnerLocal, error) {
	return &CmdRunnerLocal{printLog: true}, nil
}

// NewLoglessCmdRunner creates a new command runner for local test, which wouldn't print logs.
func NewLoglessCmdRunner() (*CmdRunnerLocal, error) {
	return &CmdRunnerLocal{printLog: false}, nil
}

// Run implements the one of hwsec.CmdRunner.
func (r *CmdRunnerLocal) Run(ctx context.Context, cmd string, args ...string) ([]byte, error) {
	if r.printLog {
		testing.ContextLogf(ctx, "Running: %s", shutil.EscapeSlice(append([]string{cmd}, args...)))
	}
	result, err := testexec.CommandContext(ctx, cmd, args...).Output()
	if e, ok := err.(*exec.ExitError); ok && e.Exited() {
		err = &hwsec.ExitError{
			errors.Wrapf(err, "|%v| failed", cmd),
			e.ExitCode(),
		}
	}
	return result, err
}
