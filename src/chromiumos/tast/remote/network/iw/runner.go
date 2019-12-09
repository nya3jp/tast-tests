// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package iw contains utility functions to wrap around the iw program.
package iw

import (
	"context"

	"chromiumos/tast/common/network/iw"
	"chromiumos/tast/dut"
)

type remoteCmdRunner struct {
	host *dut.DUT // TODO(crbug.com/1019537): use a more suitable ssh object instead of DUT as it may also be run on AP.
}

func (r *remoteCmdRunner) Run(ctx context.Context, cmd string, args ...string) error {
	return r.host.Command(cmd, args...).Run(ctx)
}

func (r *remoteCmdRunner) Output(ctx context.Context, cmd string, args ...string) ([]byte, error) {
	return r.host.Command(cmd, args...).Output(ctx)
}

// Runner is an alias for common iw Runner but only for remote execution.
type Runner = iw.Runner

// NewRunner creates a iw runner for remote execution.
func NewRunner(dut *dut.DUT) *Runner {
	return iw.NewRunner(&remoteCmdRunner{
		host: dut,
	})
}
