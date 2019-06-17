// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

/*
This file implements miscellaneous and unsorted helpers.
*/

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

// Call executes |cmd| on the DUT with |args| no matter if it's remote or local test.
// It returns the stdout and the error returned by the command, if any.
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
		return d.Command("sh", "-c", cmd).Output(ctx)
	}
	cmdToRun := cmd + " " + strings.Join([]string(args), " ")
	testing.ContextLog(ctx, "Running "+cmdToRun)
	return testexec.CommandContext(ctx, cmd, args...).Output()
}

// Reboot reboots the DUT; it does some hwsec-specific operations, including collecting coverage profiles.
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
	return d.Reboot(ctx)
}
