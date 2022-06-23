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
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
)

// LocalCmdRunner is the object used for running local commands.
type LocalCmdRunner struct {
	NoLogOnError bool // Default false: dump log on error.
}

var _ cmd.Runner = (*LocalCmdRunner)(nil)
var command *testexec.Cmd

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

// Create creates a command.
func (r *LocalCmdRunner) Create(ctx context.Context, cmd string, args ...string) {
	command = testexec.CommandContext(ctx, cmd, args...)
}

// SetStdOut sets the standard output of existed command.
func (r *LocalCmdRunner) SetStdOut(stdoutFile *os.File) {
	command.Stdout = stdoutFile
}

// StderrPipe sets standard error pipe of existed command.
func (r *LocalCmdRunner) StderrPipe() (io.ReadCloser, error) {
	return command.StderrPipe()
}

// StartExistedCmd starts existed command.
func (r *LocalCmdRunner) StartExistedCmd() error {
	if command == nil {
		return errors.New("there is no command object to start")
	}
	return command.Start()
}

// WaitExistedCmd waits existed command.
func (r *LocalCmdRunner) WaitExistedCmd() error {
	if command == nil {
		return errors.New("there is no command object to wait")
	}
	command.Wait()
	return nil
}

// CheckCmdExisted check if the command existed.
func (r *LocalCmdRunner) CheckCmdExisted() bool {
	return command != nil
}
