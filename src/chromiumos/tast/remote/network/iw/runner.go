// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package iw contains utility functions to wrap around the iw program.
package iw

import (
	"context"

	"chromiumos/tast/common/network/iw"
	"chromiumos/tast/shutil"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

// remoteCmdRunner implements iw.CmdRunner interface.
type remoteCmdRunner struct {
	host *ssh.Conn
}

var _ iw.CmdRunner = (*remoteCmdRunner)(nil)

func (r *remoteCmdRunner) Run(ctx context.Context, cmd string, args ...string) error {
	_, err := r.Output(ctx, cmd, args...)
	return err
}

func (r *remoteCmdRunner) Output(ctx context.Context, cmd string, args ...string) ([]byte, error) {
	out, err := r.host.Command(cmd, args...).Output(ctx)
	// NB: the 'local' variant uses DumpLogOnError, which is not provided by Commander.
	if err != nil {
		testing.ContextLogf(ctx, "Command: %s %s", shutil.Escape(cmd), shutil.EscapeSlice(args))
		testing.ContextLog(ctx, "Output:\n", string(out)) // NOLINT
	}
	return out, err
}

// Runner is an alias for common iw Runner but only for remote execution.
type Runner = iw.Runner

// NewRunner creates a iw runner for remote execution.
func NewRunner(host *ssh.Conn) *Runner {
	return iw.NewRunner(&remoteCmdRunner{
		host: host,
	})
}
