// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package iw contains utility functions to wrap around the iw program.
package iw

import (
	"context"

	"chromiumos/tast/common/network/iw"
	"chromiumos/tast/local/testexec"
)

type localCmdRunner struct{}

func (r *localCmdRunner) Run(ctx context.Context, cmd string, args ...string) error {
	return testexec.CommandContext(ctx, cmd, args...).Run(testexec.DumpLogOnError)
}

func (r *localCmdRunner) Output(ctx context.Context, cmd string, args ...string) ([]byte, error) {
	return testexec.CommandContext(ctx, cmd, args...).Output(testexec.DumpLogOnError)
}

// Runner is an alias for common iw Runner but only for local execution.
type Runner = iw.Runner

// NewRunner creates a iw runner for local execution.
func NewRunner() *Runner {
	return iw.NewRunner(&localCmdRunner{})
}
