// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cmd contains the interface for running commands in packages such as iw and ping.
package cmd

import (
	"context"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/remote/network/commander"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

// LocalCmdRunner is the object used for running local commands.
type LocalCmdRunner struct{}

// Run starts a local command.
func (r *LocalCmdRunner) Run(ctx context.Context, cmd string, args ...string) error {
	return testexec.CommandContext(ctx, cmd, args...).Run(testexec.DumpLogOnError)
}

// Output starts a local command and returns its output.
func (r *LocalCmdRunner) Output(ctx context.Context, cmd string, args ...string) ([]byte, error) {
	return testexec.CommandContext(ctx, cmd, args...).Output(testexec.DumpLogOnError)
}

// RemoteCmdRunner is the object used for running remote commands.
type RemoteCmdRunner struct {
	Host commander.Commander
}

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

// Runner is the shared interface for local/remote command execution.
type Runner interface {
	Run(ctx context.Context, cmd string, args ...string) error
	Output(ctx context.Context, cmd string, args ...string) ([]byte, error)
}
