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

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/testing"

	"github.com/shirou/gopsutil/process"
)

const checkDumpsPollInterval = 100 * time.Millisecond

// getAllMinidumps returns a map keyed by paths of all Chrome minidump files.
func getAllMinidumps() (map[string]struct{}, error) {
	_, dumps, err := crash.GetCrashes(crash.ChromeCrashDir)
	if err != nil {
		return nil, err
	}
	m := make(map[string]struct{})
	for _, d := range dumps {
		m[d] = struct{}{}
	}
	return m, nil
}

// getNewMinidumps returns paths of current Chrome minidumps not present in old,
// which should've been created via an earlier call to getAllMinidumps.
func getNewMinidumps(old map[string]struct{}) ([]string, error) {
	dumps := make([]string, 0)
	if ds, err := getAllMinidumps(); err != nil {
		return nil, err
	} else {
		for d := range ds {
			if _, ok := old[d]; ok {
				continue
			}
			dumps = append(dumps, d)
		}
	}
	return dumps, nil
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
func KillAndGetDumps(ctx context.Context) ([]string, error) {
	od, err := getAllMinidumps()
	if err != nil {
		return nil, fmt.Errorf("failed to get Chrome minidumps: %v", err)
	}

	pids, err := chrome.GetPIDs()
	if err != nil {
		return nil, fmt.Errorf("failed to get Chrome PIDs: %v", err)
	}

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

	// Remove the new dumps so they don't get included in the test results.
	nd, err := getNewMinidumps(od)
	if err != nil {
		return nil, fmt.Errorf("failed getting new minidumps: %v", err)
	}
	for _, p := range nd {
		testing.ContextLog(ctx, "Deleting (expected) new minidump: ", p)
		os.Remove(p)
	}
	return nd, nil
}
