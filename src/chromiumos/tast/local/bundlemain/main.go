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
	"time"

	"chromiumos/tast/bundle"
	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/local/disk"
	"chromiumos/tast/local/faillog"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/local/ready"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/upstart"
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

func hwsecResetDACounter(ctx context.Context) error {
	cmdRunner, err := hwseclocal.NewCmdRunner()
	if err != nil {
		return errors.Wrap(err, "failed to create CmdRunner")
	}

	tpmManagerUtil, err := hwsec.NewUtilityTpmManagerBinary(cmdRunner)
	if err != nil {
		return errors.Wrap(err, "failed to create UtilityTpmManagerBinary")
	}

	// Reset the TPM dictionary attack counter
	if msg, err := tpmManagerUtil.ResetDALock(ctx); err != nil {
		return errors.Wrapf(err, "failed to reset TPM dictionary attack: %s", msg)
	}
	return nil
}

func hwsecCheckDACounter(ctx context.Context) error {
	cmdRunner, err := hwseclocal.NewCmdRunner()
	if err != nil {
		return errors.Wrap(err, "failed to create CmdRunner")
	}

	tpmManagerUtil, err := hwsec.NewUtilityTpmManagerBinary(cmdRunner)
	if err != nil {
		return errors.Wrap(err, "failed to create UtilityTpmManagerBinary")
	}

	// Get the TPM dictionary attack info
	daInfo, err := tpmManagerUtil.GetDAInfo(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get the TPM dictionary attack info")
	}
	if daInfo.Counter != 0 {
		return errors.Errorf("TPM dictionary counter is not zero: %#v", daInfo)
	}
	return nil
}

func testHookLocal(ctx context.Context, s *testing.TestHookState) func(ctx context.Context, s *testing.TestHookState) {
	// Ensure the ui service is running.
	if upstart.WaitForJobStatus(ctx, "ui", upstart.StartGoal, upstart.RunningState, upstart.RejectWrongGoal, 10*time.Second) != nil {
		s.Log("Starting ui service")
		con, cancel := context.WithTimeout(ctx, 10*time.Second)
		if err := upstart.StartJob(con, "ui"); err != nil {
			s.Log("Failed to start ui service: ", err)
		}
		cancel()
	}

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

	if err := crash.MarkTestInProgress(s.TestName()); err != nil {
		s.Log("Failed to mark crash test in progress: ", err)
	}

	// Wait for Internet connectivity.
	if err := shill.WaitForOnline(ctx); err != nil {
		s.Log("Failed to wait for Internet connectivity: ", err)
	}

	// Reset the TPM dictionary attack counter before running the tast.
	if err := hwsecResetDACounter(ctx); err != nil {
		s.Log("Failed to reset TPM DA counter: ", err)
	}

	return func(ctx context.Context, s *testing.TestHookState) {

		// Ensure the TPM dictionary attack counter is zero after tast finish.
		if err := hwsecCheckDACounter(ctx); err != nil {
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

// beforeDownload is called before download external data files.
func beforeDownload(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// Ensure networking is up.
	if err := shill.WaitForOnline(ctx); err != nil {
		testing.ContextLog(ctx, "Before downloading external data files: failed to wait for online: ", err)
	}
}

// RunLocal is an entry point function for local bundles.
func RunLocal() {
	os.Exit(bundle.LocalDefault(bundle.LocalDelegate{
		Ready:          ready.Wait,
		TestHook:       testHookLocal,
		BeforeDownload: beforeDownload,
	}))
}
