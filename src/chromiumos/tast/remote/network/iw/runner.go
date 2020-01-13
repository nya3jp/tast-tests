// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package iw contains utility functions to wrap around the iw program.
package iw

import (
	"context"

	"chromiumos/tast/common/network/iw"
	"chromiumos/tast/remote/network/utils"
)

// remoteCmdRunner implements iw.CmdRunner interface.
type remoteCmdRunner struct {
	host utils.Commander
}

var _ iw.CmdRunner = (*remoteCmdRunner)(nil)

func (r *remoteCmdRunner) Run(ctx context.Context, cmd string, args ...string) error {
	return r.host.Command(cmd, args...).Run(ctx)
}

func (r *remoteCmdRunner) Output(ctx context.Context, cmd string, args ...string) ([]byte, error) {
	return r.host.Command(cmd, args...).Output(ctx)
}

// Runner is an alias for common iw Runner but only for remote execution.
type Runner = iw.Runner

// NewRunner creates a iw runner for remote execution.
func NewRunner(host utils.Commander) *Runner {
	return iw.NewRunner(&remoteCmdRunner{
		host: host,
	})
}
