// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"strings"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/testexec"
)

const tpmManagerLocalDataBackupPath = "/tmp/tast-system-backup-local_tpm_data"

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
	if err := fsutil.CopyFile(hwsec.TPMManagerLocalDataPath, tpmManagerLocalDataBackupPath); err != nil {
		return errors.Wrap(err, "failed to copy tpm manager local data")
	}
	return nil
}

// RestoreTPMManagerData copies the backup file back to the location of tpm manager local data.
func RestoreTPMManagerData(ctx context.Context) error {
	if err := fsutil.CopyFile(tpmManagerLocalDataBackupPath, hwsec.TPMManagerLocalDataPath); err != nil {
		return errors.Wrap(err, "failed to copy tpm manager local data backup")
	}
	return nil
}
