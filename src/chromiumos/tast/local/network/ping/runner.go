// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ping contains utility functions to wrap around the ping program.
package ping

import (
	"context"
	"os/exec"
	"syscall"

	"chromiumos/tast/common/network/ping"
	"chromiumos/tast/local/testexec"
)

// localCmdRunner is the object used for run ping command.
type localCmdRunner struct{}

var _ ping.CmdRunner = (*localCmdRunner)(nil)

func (r *localCmdRunner) Output(ctx context.Context, cmd string, args ...string) ([]byte, error) {
	command := testexec.CommandContext(ctx, cmd, args...)
	output, err := command.Output(testexec.DumpLogOnError)
	if exitError, ok := err.(*exec.ExitError); ok {
		ws := exitError.Sys().(syscall.WaitStatus)
		ping.ExitCode = ws.ExitStatus()
	}

	return output, err
}

// Runner is an alias for common ping Runner but only for local execution.
type Runner = ping.Runner

// NewRunner creates a ping Runner on the given dut for local execution.
func NewRunner() *Runner {
	return ping.NewRunner(&localCmdRunner{})
}
