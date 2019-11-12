// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"os"
	"regexp"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/session"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
)

const systemKeyBackupFile = "/mnt/stateful_partition/unencrypted/preserve/system.key"

// Jobs/daemons that need to be stopped before we soft-clear the TPM reset system states and started afterwards.
//
// The order those jobs start matters. Make sure you know what you are doing before modifying this slice.
var JobsToRestart = []string {
	"tpm_managerd", "chapsd", "bootlockboxd", "attestationd", "u2fd", "cryptohomed", "ui",
}

// OOBE TPM-related files that should be cleared after TPM is soft-cleared.
var filesToRemove = []string {
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
var dirsToRemove = []string {
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

// Gets TPM version number from tpmc and returns the result. If the tpmc command fails, an error is returned instead.
func GetTpmVersion(ctx context.Context) (version string, err error) {
	out, err := testexec.CommandContext(ctx, "tpmc", "tpmver").Output()

	// Trailing newline char is trimmed.
	return strings.TrimSpace(string(out)), err
}

// Soft-clears the TPM, clears OOBE, device ownership, and TPM-related states, and restarts UI and
// TPM-related daemons.
// System key used by encstateful is restored after TPM is soft-cleared.
func ResetTpmAndSystemStates(ctx context.Context) []error {
	// Make sure this is a TPM 2.0 device.
	version, err := GetTpmVersion(ctx)
	if err != nil {
		return []error{errors.Wrap(err, "Failed to get TPM version.")}
	} else if version != "2.0" {
		return []error{
			errors.Errorf("We don't support TPM version %s for TPM soft-clearing yet.", version)}
	}

	// Check if system key exists in NVRAM
	hasSysKey, err := hasSystemKey(ctx)
	if err != nil {
		return []error{errors.Wrap(err, "Failed to check system key in NVRAM.")}
	}

	// Check if we have system key backup
	hasSysKeyBackup, err := hasSystemKeyBackup()
	if err != nil {
		return []error{errors.Wrap(err, "Failed to check the system key backup file.")}
	}

	if hasSysKey && !hasSysKeyBackup {
		return []error{errors.New(
			"There is a system key but not its backup. We shouldn't soft-clear the TPM.")}
	}

	var retErrs []error

	// Stops ui and all hwsec daemons except for trunksd before soft-clearing the
	// TPM so that they don't run into weird states.
	//
	// trunksd is needed by the tpm_softclear command below and is stopped/started
	// separately.
	daemonsToStop := reverseStringSlice(JobsToRestart)
	err = stopJobs(ctx, daemonsToStop)
	defer func() {
		if err = ensureJobsStarted(ctx, JobsToRestart); err != nil {
			retErrs = append(retErrs, err)
		}
	}()
	if err != nil {
		retErrs = append(retErrs, err)
		return retErrs
	}

	// Actually clears the TPM
	err = testexec.CommandContext(ctx, "tpm_softclear").Run()
	if err != nil {
		retErrs = append(retErrs, errors.Wrap(err, "Failed to soft-clear TPM."))
		return retErrs
	}

	trunksd := []string{ "trunksd" }
	err = stopJobs(ctx, trunksd)
	defer func() {
		if err = ensureJobsStarted(ctx, trunksd); err != nil {
			retErrs = append(retErrs, err)
		}
	}()
	if err != nil {
		retErrs = append(retErrs, err)
		return retErrs
	}

	if hasSysKey {
		if err = restoreSystemKey(ctx); err != nil {
			retErrs = append(retErrs, err)

			// Continue to reset daemons and system states even if we fail to restore system key, since
			// the TPM is already cleared.
		}
	}

	if err = resetDaemonsAndSystemStates(ctx); err != nil {
		retErrs = append(retErrs, err)
	}
	return retErrs
}

// Returns if the system key for encstateful exists in NVRAM or an error on any failure.
func hasSystemKey(ctx context.Context) (spaceExists bool, err error) {
	var nvSpaceInfoRegexp = regexp.MustCompile(`(?m)^\s*result:\s*NVRAM_RESULT_SUCCESS\s*$`)

	out, err := testexec.CommandContext(
		ctx, "tpm_manager_client", "get_space_info", "--index=0x800005").Output()
	return nvSpaceInfoRegexp.Match(out), err
}

// Returns if the system key backup exists on the disk or an error on any failure.
func hasSystemKeyBackup() (backupExists bool, err error) {
	fileInfo, err := os.Stat(systemKeyBackupFile)

	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}

		return false, err
	} else if fileInfo.IsDir() {
		return false, errors.Errorf("%s is a dir !!", systemKeyBackupFile)
	}

	return true, nil
}

// Restores encstateful system key to NVRAM.
func restoreSystemKey(ctx context.Context) error {
	if err := testexec.CommandContext(
		ctx, "mount-encrypted", "set", systemKeyBackupFile).Run(); err != nil {
		return errors.Wrapf(err, "Failed to restore system key into NVRAM from %s.", systemKeyBackupFile)
	}

	return nil
}

// Ensures the given jobs are started or return an error if any one of the jobs fails to start.
func ensureJobsStarted(ctx context.Context, jobs[] string) error {
	for _, job := range jobs {
		if err := upstart.EnsureJobRunning(ctx, job); err != nil {
			return errors.Wrapf(err, "Failed to start %s.", job)
		}
	}

	return nil
}

// Ensures the given jobs are stopped or return an error if any one of the jobs fails to stop. It's
// not an error if a given job is already stopped.
func stopJobs(ctx context.Context, jobs[] string) error {
	for _, job := range jobs {
		if err := upstart.StopJob(ctx, job); err != nil {
			return errors.Wrapf(err, "Failed to stop %s.", job)
		}
	}

	return nil
}

// Returns the reverse of the input slice. This function doesn't change the input slice.
func reverseStringSlice(elements[] string) []string {
	length := len(elements)
	newElements := make([]string, length)

	for i := 0; i < length; i++ {
		newElements[i] = elements[length - i - 1]
	}

	return newElements
}

// Removes TPM-related caches and device policies and states
func resetDaemonsAndSystemStates(ctx context.Context) error {
	for _, filename := range filesToRemove {
		if err := os.Remove(filename); err != nil && !os.IsNotExist(err) {
			return errors.Wrapf(err, "Failed to remove file %s.", filename)
		}
	}

	for _, dirname := range dirsToRemove {
		if err := os.RemoveAll(dirname); err != nil {
			return errors.Wrapf(err, "Failed to remove dir %s.", dirname)
		}
	}

	// Clears /var/lib/whitelist and /home/chronos/Local States.
	return session.ClearDeviceOwnership(ctx)
}
