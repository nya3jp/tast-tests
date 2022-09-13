// Copyright 2020 The ChromiumOS Authors
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

// RemoteCmdRunner is the object used for running remote commands.
type RemoteCmdRunner struct {
	Host         *ssh.Conn
	NoLogOnError bool // Default false: dump log on error.
	command      *ssh.Cmd
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

// CreateCmd creates a command.
func (r *RemoteCmdRunner) CreateCmd(ctx context.Context, cmd string, args ...string) {
	r.command = r.Host.CommandContext(ctx, cmd, args...)
}

// SetStdOut sets the standard output of existed command.
func (r *RemoteCmdRunner) SetStdOut(stdoutFile *os.File) {
	r.command.Stdout = stdoutFile
}

// StderrPipe sets standard error pipe of existed command.
func (r *RemoteCmdRunner) StderrPipe() (io.ReadCloser, error) {
	return r.command.StderrPipe()
}

// StartCmd starts a command that is created by r.Create().
func (r *RemoteCmdRunner) StartCmd() error {
	if !r.CmdExists() {
		return errors.New("there is no command object to start")
	}
	return r.command.Start()
}

// WaitCmd waits a command that is created by r.Create().
func (r *RemoteCmdRunner) WaitCmd() error {
	if !r.CmdExists() {
		return errors.New("there is no command object to wait")
	}
	r.command.Wait()
	return nil
}

// CmdExists check if the command is created by r.Create().
func (r *RemoteCmdRunner) CmdExists() bool {
	return r.command != nil
}

// ReleaseProcess detaches the command process from the parent process and keeps it alive
// after the parent process dies.
// The function only works for local test environment.
func (r *RemoteCmdRunner) ReleaseProcess() error {
	return nil
}

// ResetCmd sets the command to null.
func (r *RemoteCmdRunner) ResetCmd() {
	r.command = nil
}
