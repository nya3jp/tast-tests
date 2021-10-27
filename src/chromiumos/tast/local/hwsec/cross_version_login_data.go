
// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"strings"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
)

func CreateCrossVersionLoginData(ctx context.Context, daemonController *hwsec.DaemonController, filePath string) (retErr error) {
	if err := daemonController.TryStopDaemons(ctx, hwsec.HighLevelTPMDaemons); err != nil {
    return errors.Wrap(err, "failed to try to stop high-level TPM daemons")
	}
	if err := daemonController.TryStopDaemons(ctx, hwsec.LowLevelTPMDaemons); err != nil {
    return errors.Wrap(err, "failed to try to stop low-level TPM daemons")
	}
  defer func() {
    if err := daemonController.EnsureDaemons(ctx, hwsec.LowLevelTPMDaemons); err != nil {
      retErr = errors.Wrap(err, "failed to ensure low-level TPM daemons")
      return
    }
    if err := daemonController.EnsureDaemons(ctx, hwsec.HighLevelTPMDaemons); err != nil {
      retErr = errors.Wrap(err, "failed to ensure high-level TPM daemons")
      return
    }
  }()

  output, err := testexec.CommandContext(ctx, "find", "/home/.shadow/", "-maxdepth", "2", "-type", "f").Output()
  if err != nil {
    return errors.Wrap(err, "failed to find the cryptohome data")
  }
  data := strings.Split(string(output), "\n")

  args := []string {
    "Jcvf",
    filePath,
    "/mnt/stateful_partition/unencrypted/tpm2-simulator/NVChip",
    "/var/lib/tpm_manager/local_tpm_data",
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

func LoadCrossVersionLoginData(ctx context.Context, daemonController *hwsec.DaemonController, filePath string) (retErr error) {
	if err := daemonController.TryStopDaemons(ctx, hwsec.HighLevelTPMDaemons); err != nil {
    return errors.Wrap(err, "failed to try to stop high-level TPM daemons")
	}
	if err := daemonController.TryStopDaemons(ctx, hwsec.LowLevelTPMDaemons); err != nil {
    return errors.Wrap(err, "failed to try to stop low-level TPM daemons")
	}
  defer func() {
    if err := daemonController.EnsureDaemons(ctx, hwsec.LowLevelTPMDaemons); err != nil {
      retErr = errors.Wrap(err, "failed to ensure low-level TPM daemons")
      return
    }
    if err := daemonController.EnsureDaemons(ctx, hwsec.HighLevelTPMDaemons); err != nil {
      retErr = errors.Wrap(err, "failed to ensure high-level TPM daemons")
      return
    }
  }()

  if err := testexec.CommandContext(ctx, "rm", "-rf", "/home/.shadow").Run(); err != nil {
    return errors.Wrap(err, "failed to remove old data")
  }
  if err := testexec.CommandContext(ctx, "tar", "Jxvf", filePath, "-C", "/").Run(); err != nil {
    return errors.Wrap(err, "failed to decompress the cryptohome data")
  }
  return nil
}
