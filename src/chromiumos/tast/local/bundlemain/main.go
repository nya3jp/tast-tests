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
	"os"
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
	"chromiumos/tast/local/syslog"
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
	cmdRunner := hwseclocal.NewLoglessCmdRunner()
	tpmManager := hwsec.NewTPMManagerClient(cmdRunner)

	// Get the TPM dictionary attack info
	daInfo, err := tpmManager.GetDAInfo(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get the TPM dictionary attack info")
	}
	return daInfo.Counter, nil
}

func hwsecGetTPMStatus(ctx context.Context) (*hwsec.NonsensitiveStatusInfo, error) {
	cmdRunner := hwseclocal.NewLoglessCmdRunner()
	tpmManager := hwsec.NewTPMManagerClient(cmdRunner)

	// Get the TPM nonsensitive status info
	status, err := tpmManager.GetNonsensitiveStatusIgnoreCache(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the TPM nonsensitive status info")
	}
	return status, nil
}

func hwsecCheckTPMState(ctx context.Context, origStatus *hwsec.NonsensitiveStatusInfo, origCounter int) error {
	status, err := hwsecGetTPMStatus(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get TPM status")
	}

	// We didn't expect the TPM is owned but doesn't have the permission to reset DA counter.
	if status.IsOwned && !status.HasResetLockPermissions {
		testing.ContextLog(ctx, "TPM is owned but doesn't have the permission to reset DA counter")
		// But don't failed the tast if it's not cause by this tast.
		if origStatus == nil || !origStatus.IsOwned || origStatus.HasResetLockPermissions {
			return errors.Errorf("unexpect TPM status: %#v -> %#v", origStatus, status)
		}
	}

	// Only Check the DA counter when the TPM is owned.
	if status.IsOwned {
		da, err := hwsecGetDACounter(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get DA counter")
		}
		if da > origCounter {
			return errors.Errorf("TPM dictionary counter is increased: %v -> %v", origCounter, da)
		}
	}

	return nil
}

func testHookLocal(ctx context.Context, s *testing.TestHookState) func(ctx context.Context, s *testing.TestHookState) {
	endLogFn, err := syslog.CollectSyslog()
	if err != nil {
		s.Log("Saving log position: ", err)
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

	// Store current DA value before running the tast.
	hwsecDACounter, err := hwsecGetDACounter(ctx)
	if err != nil {
		s.Log("Failed to get TPM DA counter: ", err)
		// Assume the counter value is zero when we failed to get the DA counter.
		hwsecDACounter = 0
	}

	// Store current TPM status before running the tast.
	hwsecTpmStatus, err := hwsecGetTPMStatus(ctx)
	if err != nil {
		s.Log("Failed to get TPM status: ", err)
		hwsecTpmStatus = nil
	}

	return func(ctx context.Context, s *testing.TestHookState) {

		// Ensure the TPM is in the expect state after tast finish.
		if err := hwsecCheckTPMState(ctx, hwsecTpmStatus, hwsecDACounter); err != nil {
			s.Error("Failed to check TPM state: ", err)
		}

		if s.HasError() {
			faillog.Save(ctx)
		}

		if endLogFn != nil {
			if err := endLogFn(ctx, s.OutDir()); err != nil {
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

func runHookLocal(ctx context.Context) (func(context.Context) error, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Set the upstart log priority to info, which enables upstart job state
	// transition logs.
	if err := upstart.SetLogPriority(ctx, upstart.LogPriorityInfo); err != nil {
		return nil, err
	}
	return nil, nil
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
	os.Exit(bundle.LocalDefault(bundle.Delegate{
		Ready:          ready.Wait,
		TestHook:       testHookLocal,
		RunHook:        runHookLocal,
		BeforeDownload: beforeDownload,
	}))
}
