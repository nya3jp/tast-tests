// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package bundlemain provides a main function implementation for a bundle
// to share it from various local bundle executables.
// The most of the frame implementation is in chromiumos/tast/bundle package,
// but some utilities, which lives in support libraries for maintenance,
// need to be injected.
package bundlemain

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"chromiumos/tast/bundle"
	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/local/disk"
	"chromiumos/tast/local/faillog"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/local/ready"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

const (
	varLogMessages    = "/var/log/messages"
	statefulPartition = "/mnt/stateful_partition"

	mib                 = 1024 * 1024
	lowSpaceThreshold   = 100 * mib
	spaceUsageThreshold = 10 * mib
)

func copyLogs(ctx context.Context, oldInfo os.FileInfo, outDir string) error {
	dp := filepath.Join(outDir, "messages")

	df, err := os.Create(dp)
	if err != nil {
		return errors.Wrapf(err, "failed to write log: failed to create %s", dp)
	}
	defer df.Close()

	sf, err := os.Open(varLogMessages)
	if err != nil {
		return errors.Wrapf(err, "failed to read log: failed to open %s", varLogMessages)
	}
	defer sf.Close()

	info, err := sf.Stat()
	if err != nil {
		return errors.Wrapf(err, "failed reading log position: failed to stat %s", varLogMessages)
	}

	if os.SameFile(info, oldInfo) {
		// If the file has not rotated just copy everything since the test started.
		if _, err = sf.Seek(oldInfo.Size(), 0); err != nil {
			return errors.Wrapf(err, "failed to read log: failed to seek %s", varLogMessages)
		}

		if _, err = io.Copy(df, sf); err != nil {
			return errors.Wrapf(err, "failed to write log: failed to copy %s", varLogMessages)
		}
	} else {
		// If the log has rotated copy the old file from where the test started and then copy the entire new file.
		// We assume that the log does not rotate twice during one test.
		// If we fail to open the older log, we still copy the newer one.
		previousLog := varLogMessages + ".1"

		sfp, err := os.Open(previousLog)
		if err != nil {
			_, _ = io.Copy(df, sf)

			return errors.Wrapf(err, "failed to read log: failed to open %s", previousLog)
		}
		defer sfp.Close()

		if _, err = sfp.Seek(oldInfo.Size(), 0); err != nil {
			_, _ = io.Copy(df, sf)

			return errors.Wrapf(err, "failed to read log: failed to seek %s", previousLog)
		}

		// Copy previous log
		if _, err = io.Copy(df, sfp); err != nil {
			_, _ = io.Copy(df, sf)

			return errors.Wrapf(err, "failed to write log: failed to copy previous %s", previousLog)
		}

		// Copy current log
		if _, err = io.Copy(df, sf); err != nil {
			return errors.Wrapf(err, "failed to write log: failed to copy current %s", previousLog)
		}
	}

	return nil
}

func ensureDiskSpace(ctx context.Context, purgeable []string) (uint64, error) {
	// Unconditionally delete core dumps.
	if err := crash.DeleteCoreDumps(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to delete core dumps: ", err)
	}

	// Delete purgeable files until the free space gets more than lowSpaceThreshold.
	for _, path := range purgeable {
		free, err := disk.FreeSpace(statefulPartition)
		if err != nil {
			return 0, err
		}
		if free >= lowSpaceThreshold {
			return free, nil
		}
		if err := os.Remove(path); err != nil {
			testing.ContextLog(ctx, "Failed to remove a purgeable file: ", err)
		} else {
			testing.ContextLog(ctx, "Deleted ", path)
		}
	}
	return disk.FreeSpace(statefulPartition)
}

func hwsecGetDACounter(ctx context.Context) (int, error) {
	cmdRunner, err := hwseclocal.NewCmdRunner()
	if err != nil {
		return 0, errors.Wrap(err, "failed to create CmdRunner")
	}

	tpmManagerUtil, err := hwsec.NewUtilityTpmManagerBinary(cmdRunner)
	if err != nil {
		return 0, errors.Wrap(err, "failed to create UtilityTpmManagerBinary")
	}

	// Get the TPM dictionary attack info
	daInfo, err := tpmManagerUtil.GetDAInfo(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get the TPM dictionary attack info")
	}
	return daInfo.Counter, nil
}

func hwsecGetTPMStatus(ctx context.Context) (*hwsec.NonsensitiveStatusInfo, error) {
	cmdRunner, err := hwseclocal.NewCmdRunner()
	if err != nil {
		return &hwsec.NonsensitiveStatusInfo{},
			errors.Wrap(err, "failed to create CmdRunner")
	}

	tpmManagerUtil, err := hwsec.NewUtilityTpmManagerBinary(cmdRunner)
	if err != nil {
		return &hwsec.NonsensitiveStatusInfo{},
			errors.Wrap(err, "failed to create UtilityTpmManagerBinary")
	}

	// Get the TPM nonsensitive status info
	status, err := tpmManagerUtil.GetNonsensitiveStatus(ctx)
	if err != nil {
		return &hwsec.NonsensitiveStatusInfo{},
			errors.Wrap(err, "failed to get the TPM nonsensitive status info")
	}
	return status, nil
}

func hwsecCheckDACounter(ctx context.Context, origVal int) error {
	da, err := hwsecGetDACounter(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get DA counter")
	}
	if da > origVal {
		return errors.Errorf("TPM dictionary counter is increased: %v -> %v", origVal, da)
	}
	return nil
}

func hwsecCheckTPMStatus(ctx context.Context, s *testing.TestHookState, origStatus *hwsec.NonsensitiveStatusInfo) error {
	status, err := hwsecGetTPMStatus(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get TPM status")
	}
	// We didn't expect the TPM is owned but doesn't have the permission to reset DA counter.
	if status.IsOwned && !status.HasResetLockPermissions {
		s.Log("TPM is owned but doesn't have the permission to reset DA counter")
		// But don't failed the tast if it's not cause by this tast.
		if !origStatus.IsOwned || origStatus.HasResetLockPermissions {
			return errors.Errorf("Unexpect TPM status: %#v -> %#v", origStatus, status)
		}
	}
	return nil
}

func testHookLocal(ctx context.Context, s *testing.TestHookState) func(ctx context.Context, s *testing.TestHookState) {
	// Store the current log state.
	oldInfo, err := os.Stat(varLogMessages)
	if err != nil {
		s.Logf("Saving log position: failed to stat %s: %v", varLogMessages, err)
		oldInfo = nil
	}

	// Ensure disk space and record the current free space.
	checkFreeSpace := false
	freeSpaceBefore, err := ensureDiskSpace(ctx, s.Purgeable())
	if err != nil {
		s.Log("Failed to ensure disk space: ", err)
	} else {
		checkFreeSpace = true
		if freeSpaceBefore < lowSpaceThreshold {
			s.Logf("Low disk space before starting test: %d MiB available", freeSpaceBefore/mib)
		}
	}

	if err := crash.MarkTestInProgress(s.TestInstance().Name); err != nil {
		s.Log("Failed to mark crash test in progress: ", err)
	}

	// Wait for Internet connectivity.
	if err := shill.WaitForOnline(ctx); err != nil {
		s.Log("Failed to wait for Internet connectivity: ", err)
	}

	// Store current DA value before running the tast.
	hwsecDACounter, err := hwsecGetDACounter(ctx)
	if err != nil {
		s.Log("Failed to get TPM DA counter: ", err)
	}

	// Store current TPM status before running the tast.
	hwsecTpmStatus, err := hwsecGetTPMStatus(ctx)
	if err != nil {
		s.Log("Failed to get TPM status: ", err)
	}

	return func(ctx context.Context, s *testing.TestHookState) {

		// Ensure the TPM is in the expect status after tast finish.
		if err := hwsecCheckTPMStatus(ctx, hwsecTpmStatus); err != nil {
			s.Error("Failed to check TPM status: ", err)
		}

		// Ensure the TPM dictionary attack counter didn't be increased after tast finish.
		if err := hwsecCheckDACounter(ctx, s, hwsecDACounter); err != nil {
			s.Error("Failed to check TPM DA counter: ", err)
		}

		if s.HasError() {
			faillog.Save(ctx)
		}

		if oldInfo != nil {
			if err := copyLogs(ctx, oldInfo, s.OutDir()); err != nil {
				s.Log("Failed to copy logs: ", err)
			}
		}

		if err := crash.MarkTestDone(); err != nil {
			s.Log("Failed to unmark crash test in progress file: ", err)
		}

		if checkFreeSpace {
			freeSpaceAfter, err := disk.FreeSpace(statefulPartition)
			if err != nil {
				s.Log("Failed to read the amount of free disk space: ", err)
				freeSpaceAfter = freeSpaceBefore
			}

			var spaceUsage uint64
			if freeSpaceBefore < freeSpaceAfter {
				spaceUsage = 0
			} else {
				spaceUsage = freeSpaceBefore - freeSpaceAfter
			}

			if spaceUsage > spaceUsageThreshold {
				s.Logf("Stateful partition usage: %d MiB (%d MiB free -> %d MiB free)", spaceUsage/mib, freeSpaceBefore/mib, freeSpaceAfter/mib)
			}
		}
	}
}

// RunLocal is an entry point function for local bundles.
func RunLocal() {
	os.Exit(bundle.LocalDefault(bundle.LocalDelegate{
		Ready:    ready.Wait,
		TestHook: testHookLocal,
	}))
}
