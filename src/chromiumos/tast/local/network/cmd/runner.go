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
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
)

// LocalCmdRunner is the object used for running local commands.
type LocalCmdRunner struct {
	NoLogOnError bool // Default false: dump log on error.
	command      *testexec.Cmd
}

var _ cmd.Runner = (*LocalCmdRunner)(nil)

// Run runs a command and waits for its completion.
func (r *LocalCmdRunner) Run(ctx context.Context, cmd string, args ...string) error {
	cc := testexec.CommandContext(ctx, cmd, args...)
	if r.NoLogOnError {
		return cc.Run()
	}
	return cc.Run(testexec.DumpLogOnError)
}

// Output runs a command, waits for its completion and returns stdout output of the command.
func (r *LocalCmdRunner) Output(ctx context.Context, cmd string, args ...string) ([]byte, error) {
	cc := testexec.CommandContext(ctx, cmd, args...)
	if r.NoLogOnError {
		return cc.Output()
	}
	return cc.Output(testexec.DumpLogOnError)
}

// CreateCmd creates a command.
func (r *LocalCmdRunner) CreateCmd(ctx context.Context, cmd string, args ...string) {
	r.command = testexec.CommandContext(ctx, cmd, args...)
}

// SetStdOut sets the standard output of existed command.
func (r *LocalCmdRunner) SetStdOut(stdoutFile *os.File) {
	r.command.Stdout = stdoutFile
}

// StderrPipe sets standard error pipe of existed command.
func (r *LocalCmdRunner) StderrPipe() (io.ReadCloser, error) {
	return r.command.StderrPipe()
}

// StartCmd starts a command that is created by r.Create().
func (r *LocalCmdRunner) StartCmd() error {
	if !r.CmdExists() {
		return errors.New("there is no command object to start")
	}
	return r.command.Start()
}

// WaitCmd waits a command that is created by r.Create().
func (r *LocalCmdRunner) WaitCmd() error {
	if !r.CmdExists() {
		return errors.New("there is no command object to wait")
	}
	r.command.Wait()
	return nil
}

// CmdExists check if the command is created by r.Create().
func (r *LocalCmdRunner) CmdExists() bool {
	return r.command != nil
}

// ReleaseProcess detaches the command process from the parent process and keeps it alive
// after the parent process dies.
func (r *LocalCmdRunner) ReleaseProcess() error {
	return r.command.Process.Release()
}

// ResetCmd sets the command to null.
func (r *LocalCmdRunner) ResetCmd() {
	r.command = nil
}
