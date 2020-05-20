// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cmd contains the interface for running commands in packages such as iw and ping.
package cmd

import (
	"context"
	"io/ioutil"
	"path/filepath"

	"chromiumos/tast/common/network/cmd"
	"chromiumos/tast/shutil"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

const logName = "cmdOutput.txt"

// RemoteCmdRunner is the object used for running remote commands.
type RemoteCmdRunner struct {
	Host         *ssh.Conn
	NoLogOnError bool // Default false: dump log on error.
}

var _ cmd.Runner = (*RemoteCmdRunner)(nil)

// Run runs a command and waits for its completion.
func (r *RemoteCmdRunner) Run(ctx context.Context, cmd string, args ...string) error {
	_, err := r.Output(ctx, cmd, args...)
	return err
}

// Output runs a command, waits for its completion and returns stdout output of the command.
func (r *RemoteCmdRunner) Output(ctx context.Context, cmd string, args ...string) ([]byte, error) {
	out, err := r.Host.Command(cmd, args...).Output(ctx)
	// NB: the 'local' variant uses DumpLogOnError, which is not provided by Commander.
	if err != nil && !r.NoLogOnError {
		testing.ContextLogf(ctx, "Failed to run command: %s %s", shutil.Escape(cmd), shutil.EscapeSlice(args))
		dir, ok := testing.ContextOutDir(ctx)
		if !ok {
			testing.ContextLog(ctx, "Failed to open OutDir to dump command output")
			return nil, err
		}
		logPath := filepath.Join(dir, logName)
		if err := ioutil.WriteFile(logPath, out, 0644); err != nil {
			testing.ContextLog(ctx, "Failed to write command output: ", err)
		}
		testing.ContextLog(ctx, "Command output stored in ", logPath)
	}
	return out, err
}
