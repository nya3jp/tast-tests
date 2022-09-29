// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

import (
	"context"

	"golang.org/x/sys/unix"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// TPSValidationHelper is a helper to produce extra CPU/memory load.
type TPSValidationHelper struct {
	cmd *testexec.Cmd
	ctx context.Context
}

// NewTPSValidationHelper creates a new instance of TPSValidationHelper.
func NewTPSValidationHelper(ctx context.Context) *TPSValidationHelper {
	// Calculating a hash sum from an endless datastream. This can produce 20%+
	// extra CPU load for TPS validation tests.
	script := `sha1sum /dev/zero | sha1sum /dev/zero`
	cmd := testexec.CommandContext(ctx, "bash", "-c", script)
	return &TPSValidationHelper{
		cmd: cmd,
		ctx: ctx,
	}
}

// Stress starts to run the bash script to increase CPU/memory load.
func (vh *TPSValidationHelper) Stress() error {
	testing.ContextLog(vh.ctx, "producing extra CPU/RAM load")
	if err := vh.cmd.Start(); err != nil {
		return errors.Wrap(err, "failed to run the stress script")
	}
	return nil
}

// Release kills the extra load process.
func (vh *TPSValidationHelper) Release() error {
	testing.ContextLog(vh.ctx, "killing the stress process")
	if err := vh.cmd.Kill(); err != nil {
		return errors.Wrap(err, "failed to kill the stress process")
	}
	err := vh.cmd.Wait(testexec.DumpLogOnError)
	status, ok := testexec.GetWaitStatus(err)
	if ok {
		return nil
	}
	if status.Signaled() && status.Signal() == unix.SIGKILL {
		return nil
	}
	return err
}
