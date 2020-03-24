// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ping contains utility functions to wrap around the ping program.
package ping

import (
	"context"

	"chromiumos/tast/common/network/ping"
	"chromiumos/tast/remote/network/commander"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

// Option is an alias for for common ping Runner.
type Option = ping.Option

// remoteCmdRunner is the object used for run ping command.
type remoteCmdRunner struct {
	host commander.Commander
}

var _ ping.CmdRunner = (*remoteCmdRunner)(nil)

func (r *remoteCmdRunner) Output(ctx context.Context, cmd string, args ...string) ([]byte, error) {
	out, err := r.host.Command(cmd, args...).Output(ctx)
	// NB: the 'local' variant uses DumpLogOnError, which is not provided by Commander.
	if err != nil {
		testing.ContextLogf(ctx, "Command: %s %s", shutil.Escape(cmd), shutil.EscapeSlice(args))
		testing.ContextLog(ctx, "Output:\n", string(out)) // NOLINT
	}
	return out, err
}

// Runner is an alias for common ping Runner but only for remote execution.
type Runner = ping.Runner

// NewRunner creates a ping Runner on the given dut for remote execution.
func NewRunner(host commander.Commander) *Runner {
	return ping.NewRunner(&remoteCmdRunner{host: host})
}
