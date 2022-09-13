// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cmd contains the interface for running commands in packages such as iw and ping.
package cmd

import (
	"context"
	"io"
	"os"
)

// Runner is the shared interface for local/remote command execution.
type Runner interface {
	// Run runs a command and waits for its completion.
	Run(ctx context.Context, cmd string, args ...string) error
	// Output runs a command, waits for its completion and returns stdout output of the command.
	Output(ctx context.Context, cmd string, args ...string) ([]byte, error)
	// CreateCmd creates a command.
	CreateCmd(ctx context.Context, cmd string, args ...string)
	// SetStdOut sets the standard output of existed command.
	SetStdOut(stdoutFile *os.File)
	// StderrPipe sets standard error pipe of existed command.
	StderrPipe() (io.ReadCloser, error)
	// StartCmd starts a command that is created by r.Create().
	StartCmd() error
	// WaitCmd waits a command that is created by r.Create().
	WaitCmd() error
	// CmdExists check if the command is created by r.Create().
	CmdExists() bool
	// ReleaseProcess detaches the command process from the parent process and keeps it alive
	// after the parent process dies.
	// The function only works for local test environment.
	ReleaseProcess() error
	// ResetCmd sets the command to null.
	ResetCmd()
}
