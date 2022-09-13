// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"strings"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/testing"
)

const tpmManagerLocalDataBackupPath = "/var/lib/tpm_manager/local_tpm_data.tast-hwsec-backup"

// isTPMLocalDataIntact uses tpm_manager_client to check if local data still contains owner password,
// which means the set of imporant secrets are still intact.
func isTPMLocalDataIntact(ctx context.Context) (bool, error) {
	out, err := testexec.CommandContext(ctx, "tpm_manager_client", "status").Output()
	if err != nil {
		return false, errors.Wrap(err, "failed to call tpm_manager_client")
	}
	return strings.Contains(string(out), "owner_password"), nil
}

// BackupTPMManagerDataIfIntact backs up a the tpm manager data if the important secrets is not cleared.
func BackupTPMManagerDataIfIntact(ctx context.Context) error {
	ok, err := isTPMLocalDataIntact(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to check tpm local data")
	}
	if !ok {
		return errors.New("owner password not found")
	}
	if err := fsutil.CopyFile(hwsec.TpmManagerLocalDataPath, tpmManagerLocalDataBackupPath); err != nil {
		return errors.Wrap(err, "failed to copy tpm manager local data")
	}
	return nil
}

// RestoreTPMManagerData copies the backup file back to the location of tpm manager local data.
func RestoreTPMManagerData(ctx context.Context) error {
	if err := fsutil.CopyFile(tpmManagerLocalDataBackupPath, hwsec.TpmManagerLocalDataPath); err != nil {
		return errors.Wrap(err, "failed to copy tpm manager local data backup")
	}
	return nil
}

// RestoreTPMOwnerPasswordIfNeeded restores the owner password from the snapshot stored
// at the beginning of the entire test program, if the owner password got wiped already.
func RestoreTPMOwnerPasswordIfNeeded(ctx context.Context, dc *hwsec.DaemonController) error {
	hasOwnerPassword, err := isTPMLocalDataIntact(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to check owner password")
	}
	if hasOwnerPassword {
		return nil
	}
	if err := RestoreTPMManagerData(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to restore tpm manager local data")
		testing.ContextLog(ctx, "If you saw this on local testing, probably the TPM ownership isn't taken by the testing infra")
		testing.ContextLog(ctx, "You chould try to power wash the device and run the test again")
		return errors.Wrap(err, "failed to restore tpm manager local data")
	}
	if err := dc.Restart(ctx, hwsec.TPMManagerDaemon); err != nil {
		return errors.Wrap(err, "failed to restart tpm manager")
	}
	hasOwnerPassword, err = isTPMLocalDataIntact(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to check owner password")
	}
	if !hasOwnerPassword {
		return errors.Wrap(err, "no owner password after restoration")
	}
	return nil
}
