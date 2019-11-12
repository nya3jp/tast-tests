// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package hwsec contains TPM-related utility functions for local tests.
package hwsec

import (
	"context"
	"os"
	"regexp"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/session"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

const systemKeyBackupFile = "/mnt/stateful_partition/unencrypted/preserve/system.key"

// Jobs/daemons that need to be stopped/restarted before/after we soft-clear the TPM and reset system states.
//
// The order those jobs start matters. Make sure you know what you are doing before modifying this slice.
var jobsToRestart = []string{
	"tpm_managerd", "chapsd", "bootlockboxd", "attestationd", "u2fd", "cryptohomed", "ui",
}

// OOBE and TPM-related files that should be cleared after TPM is soft-cleared.
var filesToRemove = []string{
	"/mnt/stateful_partition/.tpm_owned",
	"/mnt/stateful_partition/.tpm_status",
	"/mnt/stateful_partition/.tpm_status.sum",
	"/home/.shadow/.can_attempt_ownership",
	"/home/.shadow/attestation.epb",
	"/home/.shadow/cryptohome.key",
	"/home/.shadow/cryptohome.key.sum",
	"/home/.shadow/install_attributes.pb",
	"/home/.shadow/install_attributes.pb.sum",
	"/home/.shadow/salt",
	"/home/.shadow/salt.sum",
	"/home/chronos/.oobe_completed",
	"/var/lib/public_mount_salt",
}

// Dirs where TPM-related daemons cache data/states. Those dirs should be removed after TPM is soft-cleared.
var dirsToRemove = []string{
	"/home/.shadow/low_entropy_creds",
	"/run/cryptohome",
	"/run/lockbox",
	"/run/tpm_manager",
	"/var/lib/bootlockbox",
	"/var/lib/boot-lockbox",
	"/var/lib/chaps",
	"/var/lib/cryptohome",
	"/var/lib/tpm_manager",
	"/var/lib/u2f",
}

// ResetTPMAndSystemStates soft-clears the TPM, resets the OOBE state, device ownership, and
// TPM-related states, and restarts UI and TPM-related daemons. System key used by encstateful is
// restored after TPM is soft-cleared.
//
// There might be multiple errors happening in this function, in which case only the first error
// will be returned, and all but the first error will be logged.
func ResetTPMAndSystemStates(ctx context.Context) (firstErr error) {
	// Makes sure this is a TPM 2.0 device.
	version, firstErr := GetTPMVersion(ctx)
	if firstErr != nil {
		return errors.Wrap(firstErr, "failed to get TPM version")
	} else if version != "2.0" {
		return errors.Errorf("we don't support TPM version %s for TPM soft-clearing yet", version)
	}

	// Checks if system key exists in NVRAM.
	hasSysKey, firstErr := hasSystemKey(ctx)
	if firstErr != nil {
		return errors.Wrap(firstErr, "failed to check system key in NVRAM")
	}

	// Checks if we have system key backup.
	hasSysKeyBackup, firstErr := hasSystemKeyBackup()
	if firstErr != nil {
		return errors.Wrap(firstErr, "failed to check the system key backup file")
	}

	if hasSysKey && !hasSysKeyBackup {
		return errors.New("there is a system key but not its backup; we shouldn't soft-clear the TPM")
	}

	// Logs the input error or sets it as the first error. This is to make sure all errors are logged.
	logOrUpdateFirstErr := func(err error) {
		if err == nil {
			return
		}

		if firstErr != nil {
			testing.ContextLogf(ctx, "%v", err)
		} else {
			firstErr = err
		}
	}

	// Stops ui and all hwsec daemons except for trunksd before soft-clearing the TPM so that they
	// don't run into weird states. Restarts those daemons and makes sure the cryptohome dbus service
	// is ready before returning.
	//
	// trunksd is needed by the tpm_softclear command below and is stopped/started separately.
	defer func() {
		err := ensureJobsStarted(ctx, jobsToRestart)
		logOrUpdateFirstErr(err)

		err = cryptohome.CheckService(ctx)
		logOrUpdateFirstErr(err)
	}()
	daemonsToStop := reverseStringSlice(jobsToRestart)
	if firstErr = stopJobs(ctx, daemonsToStop); firstErr != nil {
		return firstErr
	}

	// Actually clears the TPM.
	if firstErr = testexec.CommandContext(ctx, "tpm_softclear").Run(); firstErr != nil {
		return firstErr
	}

	trunksd := []string{"trunksd"}
	defer func() {
		err := ensureJobsStarted(ctx, trunksd)
		logOrUpdateFirstErr(err)
	}()
	if firstErr = stopJobs(ctx, trunksd); firstErr != nil {
		return firstErr
	}

	if hasSysKey {
		firstErr = restoreSystemKey(ctx)

		// Continues to reset daemons and system states even if we failed to restore system key, since
		// the TPM is already cleared.
	}

	err := resetDaemonsAndSystemStates(ctx)
	logOrUpdateFirstErr(err)
	return firstErr
}

// GetTPMVersion gets TPM version number from tpmc and returns the result. If the tpmc command
// fails, an error is returned instead.
func GetTPMVersion(ctx context.Context) (version string, err error) {
	out, err := testexec.CommandContext(ctx, "tpmc", "tpmver").Output()

	// Trailing newline char is trimmed.
	return strings.TrimSpace(string(out)), err
}

// hasSystemKey returns if the system key for encstateful exists in NVRAM or an error on any failure.
func hasSystemKey(ctx context.Context) (spaceExists bool, err error) {
	var nvSpaceInfoRegexp = regexp.MustCompile(`(?m)^\s*result:\s*NVRAM_RESULT_SUCCESS\s*$`)

	out, err := testexec.CommandContext(
		ctx, "tpm_manager_client", "get_space_info", "--index=0x800005").Output()
	return nvSpaceInfoRegexp.Match(out), err
}

// hasSystemKeyBackup returns if the system key backup exists on the disk or an error on any failure.
func hasSystemKeyBackup() (backupExists bool, err error) {
	fileInfo, err := os.Stat(systemKeyBackupFile)

	if os.IsNotExist(err) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	if fileInfo.IsDir() {
		return false, errors.Errorf("%s is a dir", systemKeyBackupFile)
	}

	return true, nil
}

// restoreSystemKey restores encstateful system key to NVRAM.
func restoreSystemKey(ctx context.Context) error {
	if err := testexec.CommandContext(
		ctx, "mount-encrypted", "set", systemKeyBackupFile).Run(); err != nil {
		return errors.Wrapf(err, "failed to restore system key into NVRAM from %s", systemKeyBackupFile)
	}

	return nil
}

// ensureJobsStarted ensures the given jobs are started or returns an error if any one of the jobs failed to start.
func ensureJobsStarted(ctx context.Context, jobs []string) error {
	for _, job := range jobs {
		if err := upstart.EnsureJobRunning(ctx, job); err != nil {
			return errors.Wrapf(err, "failed to start %s", job)
		}
	}

	return nil
}

// stopJobs ensures the given jobs are stopped or returns an error if any one of the jobs failed to
// stop. It's not an error if a given job is already stopped.
func stopJobs(ctx context.Context, jobs []string) error {
	for _, job := range jobs {
		if err := upstart.StopJob(ctx, job); err != nil {
			return errors.Wrapf(err, "failed to stop %s", job)
		}
	}

	return nil
}

// reverseStringSlice returns the reverse of the input slice. This function doesn't change the input slice.
func reverseStringSlice(elements []string) []string {
	length := len(elements)
	newElements := make([]string, length)

	for i := 0; i < length; i++ {
		newElements[i] = elements[length-i-1]
	}

	return newElements
}

// resetDaemonsAndSystemStates removes TPM-related caches, device policies, and local states.
func resetDaemonsAndSystemStates(ctx context.Context) error {
	for _, filename := range filesToRemove {
		if err := os.Remove(filename); err != nil && !os.IsNotExist(err) {
			return errors.Wrapf(err, "failed to remove file %s", filename)
		}
	}

	for _, dirname := range dirsToRemove {
		if err := os.RemoveAll(dirname); err != nil {
			return errors.Wrapf(err, "failed to remove dir %s", dirname)
		}
	}

	// Clears /var/lib/whitelist and /home/chronos/Local States.
	return session.ClearDeviceOwnership(ctx)
}
