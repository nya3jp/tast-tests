// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

/*
This file implements the TPM clear tool in remote tast.
*/

import (
	"context"
	"os"
	"regexp"
	"strings"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const (
	systemKeyBackupFileName = "/mnt/stateful_partition/unencrypted/preserve/system.key"
)

// TPMClearer clear the TPM via crossystem, this would only work on TPM2.0.
type TPMClearer struct {
	cmdRunner        hwsec.CmdRunner
	daemonController *hwsec.DaemonController
	hasSysKey        bool
}

// NewTPMClearer creates a new TPMClearer object, where r is used to run the command internally.
func NewTPMClearer(cmdRunner hwsec.CmdRunner, daemonController *hwsec.DaemonController) *TPMClearer {
	return &TPMClearer{cmdRunner, daemonController, false}
}

// PreClearTPM backups the system key.
func (tc *TPMClearer) PreClearTPM(ctx context.Context) error {
	// Makes sure this is a TPM 2.0 device.
	version, err := tc.getTPMVersion(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get TPM version")
	} else if version != "2.0" {
		return errors.Errorf("we don't support TPM version %s for TPM soft-clearing yet", version)
	}

	// Checks if system key exists in NVRAM.
	hasSysKey, err := tc.hasSystemKey(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to check system key in NVRAM")
	}
	tc.hasSysKey = hasSysKey

	// If system key exists, checks if we also have the system key backup.
	if tc.hasSysKey {
		if hasSysKeyBackup, err := tc.hasSystemKeyBackup(); err != nil {
			return errors.Wrap(err, "failed to check the system key backup file")
		} else if !hasSysKeyBackup {
			return errors.New("there is a system key but not its backup; we shouldn't soft-clear the TPM")
		}
	}

	return nil
}

// ClearTPM soft clears the TPM.
func (tc *TPMClearer) ClearTPM(ctx context.Context) error {
	// Using soft clear to clear the TPM
	if _, err := tc.cmdRunner.Run(ctx, "tpm_softclear"); err != nil {
		return errors.Wrap(err, "failed to soft clear the TPM")
	}

	return nil
}

// PostClearTPM restores the system key and ensures TPM daemon is up.
func (tc *TPMClearer) PostClearTPM(ctx context.Context) error {
	// Stop the TPM daemon
	if err := tc.daemonController.TryStop(ctx, hwsec.TrunksDaemon); err != nil {
		return errors.Wrap(err, "failed to try to stop TPM daemon")
	}
	defer func() {
		if err := tc.daemonController.Ensure(ctx, hwsec.TrunksDaemon); err != nil {
			testing.ContextLog(ctx, "Failed to ensure TPM daemon: ", err)
		}
	}()

	if tc.hasSysKey {
		if err := tc.restoreSystemKey(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to restore system key: ", err)
			// Continues to reset daemons and system states even if we failed to restore system key,
			// since the TPM is already cleared.
		}
	}

	return nil
}

// hasSystemKey returns if the system key for encstateful exists in NVRAM or an error on any failure.
func (tc *TPMClearer) hasSystemKey(ctx context.Context) (spaceExists bool, err error) {
	var nvSpaceInfoRegexp = regexp.MustCompile(`(?m)^\s*result:\s*NVRAM_RESULT_SUCCESS\s*$`)

	out, err := tc.cmdRunner.Run(ctx, "tpm_manager_client", "get_space_info", "--index=0x800005")
	return nvSpaceInfoRegexp.Match(out), err
}

// hasSystemKeyBackup returns if the system key backup exists on the disk or an error on any failure.
func (tc *TPMClearer) hasSystemKeyBackup() (backupExists bool, err error) {
	fileInfo, err := os.Stat(systemKeyBackupFileName)

	if os.IsNotExist(err) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	if fileInfo.IsDir() {
		return false, errors.Errorf("%s is a dir", systemKeyBackupFileName)
	}

	return true, nil
}

// restoreSystemKey restores encstateful system key to NVRAM.
func (tc *TPMClearer) restoreSystemKey(ctx context.Context) error {
	if _, err := tc.cmdRunner.Run(ctx, "mount-encrypted", "set", systemKeyBackupFileName); err != nil {
		return errors.Wrapf(err, "failed to restore system key into NVRAM from %s", systemKeyBackupFileName)
	}

	return nil
}

// getTPMVersion would rteurn the TPM version, for example: "1.2", "2.0"
func (tc *TPMClearer) getTPMVersion(ctx context.Context) (string, error) {
	out, err := tc.cmdRunner.Run(ctx, "tpmc", "tpmver")
	// Trailing newline char is trimmed.
	return strings.TrimSpace(string(out)), err
}
