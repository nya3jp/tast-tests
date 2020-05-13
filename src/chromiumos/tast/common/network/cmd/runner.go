// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cmd contains the interface for running commands in packages such as iw and ping.
package cmd

import (
	"context"
)

// Runner is the shared interface for local/remote command execution.
type Runner interface {
	// Run runs a command and waits for its completion.
	Run(ctx context.Context, cmd string, args ...string) error
	// Output runs a command, waits for its completion and returns stdout output of the command.
	Output(ctx context.Context, cmd string, args ...string) ([]byte, error)
}
