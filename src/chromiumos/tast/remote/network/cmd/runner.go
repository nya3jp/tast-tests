// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cmd contains the interface for running commands in packages such as iw and ping.
package cmd

import (
	"context"
	"io"
	"os"

	"chromiumos/tast/common/network/cmd"
	"chromiumos/tast/errors"
	"chromiumos/tast/ssh"
)

const logName = "cmdOutput.txt"

var command *ssh.Cmd

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
	cc := r.Host.CommandContext(ctx, cmd, args...)
	if r.NoLogOnError {
		return cc.Output()
	}
	return cc.Output(ssh.DumpLogOnError)
}

// Create creates a command.
func (r *RemoteCmdRunner) Create(ctx context.Context, cmd string, args ...string) {
	command = r.Host.CommandContext(ctx, cmd, args...)
}

// SetStdOut sets the standard output of existed command.
func (r *RemoteCmdRunner) SetStdOut(stdoutFile *os.File) {
	command.Stdout = stdoutFile
}

// StderrPipe sets standard error pipe of existed command.
func (r *RemoteCmdRunner) StderrPipe() (io.ReadCloser, error) {
	return command.StderrPipe()
}

// StartExistedCmd starts existed command.
func (r *RemoteCmdRunner) StartExistedCmd() error {
	if command == nil {
		return errors.New("there is no command object to start")
	}
	return command.Start()
}

// WaitExistedCmd waits existed command.
func (r *RemoteCmdRunner) WaitExistedCmd() error {
	if command == nil {
		return errors.New("there is no command object to wait")
	}
	command.Wait()
	return nil
}

// CheckCmdExisted check if the command existed.
func (r *RemoteCmdRunner) CheckCmdExisted() bool {
	return command != nil
}
