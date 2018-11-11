// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package chromecrash contains functionality shared by tests that
// exercise Chrome crash-dumping.
package chromecrash

import (
	"context"
	"os"
	"path/filepath"
	"syscall"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/crash"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// getChromeMinidumps returns all Chrome minidump files in paths.
func getChromeMinidumps(paths []string) []string {
	var dumps []string
	for _, p := range paths {
		if filepath.Dir(p) == crash.ChromeCrashDir && filepath.Ext(p) == crash.MinidumpExt {
			dumps = append(dumps, p)
		}
	}
	return dumps
}

// getNewFiles returns all paths present in cur but not in orig.
func getNewFiles(orig, cur []string) (added []string) {
	om := make(map[string]struct{}, len(orig))
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
// All new crash-related files are also deleted.
func KillAndGetDumps(ctx context.Context) ([]string, error) {
	oldFiles, err := crash.GetCrashes(crash.DefaultDirs()...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get original crashes")
	}

	pids, err := chrome.GetPIDs()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get Chrome PIDs")
	}

	// The root Chrome process (i.e. the one that doesn't have another Chrome process
	// as its parent) is the browser process. It's not sandboxed, so it should be able
	// to write a minidump file when it crashes.
	rp, err := chrome.GetRootPID()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get root Chrome PID")
	}
	testing.ContextLog(ctx, "Sending SIGSEGV to root Chrome process ", rp)
	if err = syscall.Kill(rp, syscall.SIGSEGV); err != nil {
		return nil, err
	}

	testing.ContextLogf(ctx, "Waiting for %d Chrome process(es) to exit", len(pids))
	err = testing.Poll(ctx, func(ctx context.Context) error {
		if exist, err := anyPIDsExist(pids); err != nil {
			return errors.Wrap(err, "failed checking processes")
		} else if exist {
			return errors.New("processes still exist")
		}
		return nil
	}, nil)
	if err != nil {
		return nil, errors.Wrap(err, "Chrome didn't exit")
	}
	testing.ContextLog(ctx, "All Chrome processes exited")

	newFiles, err := crash.GetCrashes(crash.DefaultDirs()...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get new crashes")
	}
	newChromeDumps := getNewFiles(getChromeMinidumps(oldFiles), getChromeMinidumps(newFiles))
	for _, p := range newChromeDumps {
		testing.ContextLog(ctx, "Found expected Chrome minidump file ", p)
	}

	// Delete all crash files produced during this test: https://crbug.com/881638
	deleteFiles(ctx, getNewFiles(oldFiles, newFiles))

	return newChromeDumps, nil
}
