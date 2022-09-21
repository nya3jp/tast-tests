// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"

	"golang.org/x/sys/unix"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
)

// FakePCAAgent performs the execution and terminiation of the fake pca agent.
type FakePCAAgent struct {
	cmd *testexec.Cmd
}

// FakePCAAgentContext creates a new FakePCAAgent where context is used when calling the commands.
func FakePCAAgentContext(ctx context.Context) *FakePCAAgent {
	return &FakePCAAgent{testexec.CommandContext(ctx, "fake_pca_agentd")}
}

// Start starts running the fake pca agent.
func (f *FakePCAAgent) Start() error {
	return f.cmd.Start()
}

// Stop signals the fake pca agent with SIGTERM as upstart does to daemons, and waits for its termination.
func (f *FakePCAAgent) Stop() error {
	if err := f.cmd.Signal(unix.SIGTERM); err != nil {
		return errors.Wrap(err, "failed signal the process")
	}
	if err := f.cmd.Wait(); err != nil {
		return errors.Wrap(err, "failed wait for shutdown")
	}
	return nil
}
