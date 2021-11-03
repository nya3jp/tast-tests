// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"strings"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// CreateCrossVersionLoginData will create the compressed file of data that is used in cross-version login test.
func CreateCrossVersionLoginData(ctx context.Context, daemonController *hwsec.DaemonController, filePath string) error {
	if err := daemonController.TryStopDaemons(ctx, hwsec.HighLevelTPMDaemons); err != nil {
		return errors.Wrap(err, "failed to try to stop high-level TPM daemons")
	}
	if err := daemonController.TryStopDaemons(ctx, hwsec.LowLevelTPMDaemons); err != nil {
		return errors.Wrap(err, "failed to try to stop low-level TPM daemons")
	}
	if err := daemonController.TryStop(ctx, hwsec.TPM2SimulatorDaemon); err != nil {
		return errors.Wrap(err, "failed to try to stop low-level TPM daemons")
	}
	defer func() {
		if err := daemonController.Ensure(ctx, hwsec.TPM2SimulatorDaemon); err != nil {
			testing.ContextLog(ctx, "Failed to ensure tpm-simulator: ", err)
		}
		if err := daemonController.EnsureDaemons(ctx, hwsec.LowLevelTPMDaemons); err != nil {
			testing.ContextLog(ctx, "Failed to ensure low-level TPM daemons: ", err)
		}
		if err := daemonController.EnsureDaemons(ctx, hwsec.HighLevelTPMDaemons); err != nil {
			testing.ContextLog(ctx, "Failed to ensure high-level TPM daemons: ", err)
		}
	}()

	// create compressed file for NVChip and `/home/.shadow`.
	output, err := testexec.CommandContext(ctx, "find", "/home/.shadow/", "-maxdepth", "2", "-type", "f").Output()
	if err != nil {
		return errors.Wrap(err, "failed to find the cryptohome data")
	}
	data := strings.Split(string(output), "\n")

	args := []string{
		"Jcvf",
		filePath,
		"/mnt/stateful_partition/unencrypted/tpm2-simulator/NVChip",
	}
	for i := range data {
		newArg := strings.TrimSpace(data[i])
		if newArg != "" {
			args = append(args, newArg)
		}
	}
	if err = testexec.CommandContext(ctx, "tar", args...).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to compress the cryptohome data")
	}

	return nil
}
