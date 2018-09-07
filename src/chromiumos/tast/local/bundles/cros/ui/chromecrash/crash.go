// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package chromecrash contains functionality shared by tests that
// exercise Chrome crash-dumping.
package chromecrash

import (
	"context"
	"fmt"
	"os"
	"syscall"
	"time"

	"chromiumos/tast/crash"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"

	"github.com/shirou/gopsutil/process"
)

const checkDumpsPollInterval = 100 * time.Millisecond

// getNewFiles returns all paths present in cur but not in orig.
func getNewFiles(orig, cur []string) (added []string) {
	om := make(map[string]struct{})
	for _, p := range orig {
		om[p] = struct{}{}
	}

	for _, p := range cur {
		if _, ok := om[p]; !ok {
			added = append(added, p)
		}
	}
	return added
}

// deleteFiles deletes the supplied paths.
func deleteFiles(ctx context.Context, paths []string) {
	for _, p := range paths {
		testing.ContextLog(ctx, "Removing new crash file ", p)
		if err := os.Remove(p); err != nil {
			testing.ContextLogf(ctx, "Unable to remove %v: %v", p, err)
		}
	}
}

// anyPIDsExist returns true if any PIDs in pids are still present.
func anyPIDsExist(pids []int) (bool, error) {
	for _, pid := range pids {
		if exists, err := process.PidExists(int32(pid)); err != nil {
			return false, err
		} else if exists {
			return true, nil
		}
	}
	return false, nil
}

// KillAndGetDumps sends SIGSEGV to the root Chrome process, waits for new minidump
// files to be written, and then deletes them and returns their paths.
// All new minidump and core files are also deleted.
func KillAndGetDumps(ctx context.Context) ([]string, error) {
	oldChromeCores, oldChromeDumps, err := crash.GetCrashes(crash.ChromeCrashDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get Chrome crashes: %v", err)
	}
	// Ignore errors here; we're just trying to be good citizens.
	oldSysCores, oldSysDumps, _ := crash.GetCrashes(crash.DefaultCrashDir)

	pids, err := chrome.GetPIDs()
	if err != nil {
		return nil, fmt.Errorf("failed to get Chrome PIDs: %v", err)
	}

	// The root Chrome process (i.e. the one that doesn't have another Chrome process
	// as its parent) is the browser process. It's not sandboxed, so it should be able
	// to write a minidump file when it crashes.
	rp, err := chrome.GetRootPID()
	if err != nil {
		return nil, fmt.Errorf("failed to get root Chrome PID: %v", err)
	}
	testing.ContextLog(ctx, "Sending SIGSEGV to root Chrome process ", rp)
	if err = syscall.Kill(rp, syscall.SIGSEGV); err != nil {
		return nil, err
	}

	testing.ContextLogf(ctx, "Waiting for %d Chrome process(es) to exit", len(pids))
	for {
		if exist, err := anyPIDsExist(pids); err != nil {
			return nil, fmt.Errorf("failed waiting for Chrome to exit: %v", err)
		} else if !exist {
			testing.ContextLog(ctx, "All Chrome processes exited")
			break
		}
		if ctx.Err() != nil {
			return nil, fmt.Errorf("Chrome processes didn't exit: %v", ctx.Err())
		}
		time.Sleep(checkDumpsPollInterval)
	}

	curChromeCores, curChromeDumps, err := crash.GetCrashes(crash.ChromeCrashDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get Chrome crashes: %v", err)
	}
	curSysCores, curSysDumps, _ := crash.GetCrashes(crash.DefaultCrashDir)

	newChromeDumps := getNewFiles(oldChromeDumps, curChromeDumps)

	// Remove the new dumps so they don't get included in the test results.
	// Also remove other crash-related files that we find: https://crbug.com/881638
	deleteFiles(ctx, newChromeDumps)
	deleteFiles(ctx, getNewFiles(oldChromeCores, curChromeCores))
	deleteFiles(ctx, getNewFiles(oldSysCores, curSysCores))
	deleteFiles(ctx, getNewFiles(oldSysDumps, curSysDumps))

	return newChromeDumps, nil
}
