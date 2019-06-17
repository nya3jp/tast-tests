// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"strings"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func isRemoteTest(ctx context.Context) bool {
	out, err := testexec.CommandContext(ctx, "which", "cryptohome").Output()
	return err != nil || len(out) == 0
}

func Call(ctx context.Context, cmd string, args ...string) ([]byte, error) {
	return call(ctx, cmd, args...)
}
func call(ctx context.Context, cmd string, args ...string) ([]byte, error) {
	if isRemoteTest(ctx) {
		d, ok := dut.FromContext(ctx)
		if !ok {
			return []byte{}, errors.New("failed to get dut")
		}
		cmd := cmd + " " + strings.Join([]string(args), " ")
		testing.ContextLog(ctx, "Running "+cmd)
		return d.Run(ctx, cmd)
	} else {
		cmdToRun := cmd + " " + strings.Join([]string(args), " ")
		testing.ContextLog(ctx, "Running "+cmdToRun)
		return testexec.CommandContext(ctx, cmd, args...).Output()
	}
}

func Reboot(ctx context.Context) error {
	return reboot(ctx)
}

func reboot(ctx context.Context) error {
	if !isRemoteTest(ctx) {
		return errors.New("reboot operation only supported in remote test")
	}
	if err := flushCoverageData(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to flush coverage data")
	}
	d, ok := dut.FromContext(ctx)
	if !ok {
		errors.New("Failed to get DUT")
	}
	// Run the reboot command in the background to avoid the DUT potentially going down before
	// success is reported over the SSH connection. Redirect all I/O streams to ensure that the
	// SSH exec request doesn't hang (see https://en.wikipedia.org/wiki/Nohup#Overcoming_hanging).
	cmd := "nohup sh -c 'sleep 2; reboot' >/dev/null 2>&1 </dev/null &"

	if _, err := d.Run(ctx, cmd); err != nil {
		return errors.Wrap(err, "Failed to reboot DUT")
	}

	if err := d.WaitUnreachable(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for DUT to become unreachable")
	}
	if err := d.WaitConnect(ctx); err != nil {
		return errors.Wrap(err, "failed to reconnect to DUT")
	}
	return nil
}
