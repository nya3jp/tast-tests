// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cmd contains the interface for running commands in packages such as iw and ping.
package cmd

import (
	"context"

	"chromiumos/tast/common/network/cmd"
	"chromiumos/tast/remote/network/commander"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

// RemoteCmdRunner is the object used for running remote commands.
type RemoteCmdRunner struct {
	Host commander.Commander
}

var _ cmd.Runner = (*RemoteCmdRunner)(nil)

// Run starts a remote command.
func (r *RemoteCmdRunner) Run(ctx context.Context, cmd string, args ...string) error {
	_, err := r.Output(ctx, cmd, args...)
	return err
}

// Output starts a remote command and returns its output.
func (r *RemoteCmdRunner) Output(ctx context.Context, cmd string, args ...string) ([]byte, error) {
	out, err := r.Host.Command(cmd, args...).Output(ctx)
	// NB: the 'local' variant uses DumpLogOnError, which is not provided by Commander.
	if err != nil {
		testing.ContextLogf(ctx, "Command: %s %s", shutil.Escape(cmd), shutil.EscapeSlice(args))
		testing.ContextLog(ctx, "Output:\n", string(out)) // NOLINT
	}
	return out, err
}
