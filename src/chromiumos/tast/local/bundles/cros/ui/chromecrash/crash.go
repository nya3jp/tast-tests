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
	"strings"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/crash"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

const (
	cryptohomePattern = "/home/chronos/u-*"
	// CryptohomeCrashPattern is a glob pattern that matches any crash directory
	// inside any user's cryptohome
	CryptohomeCrashPattern = "/home/chronos/u-*/crash"

	// TestCert is the name of a PKCS #12 format cert file, suitable for passing
	// into metrics.SetConsent().
	TestCert = "testcert.p12"
)

func cryptohomeCrashDirs(ctx context.Context) ([]string, error) {
	// The crash subdirectory may not exist yet, so we can't just do
	// filepath.Glob(CryptohomeCrashPattern) here. Instead, look for all cryptohomes
	// and manually add a /crash on the end.
	paths, err := filepath.Glob(cryptohomePattern)
	if err != nil {
		return nil, err
	}

	for i := range paths {
		paths[i] = filepath.Join(paths[i], "crash")
	}
	return paths, nil
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

// FindCrashFilesIn looks through the list of files returned from KillAndGetCrashFiles,
// expecting to find the crash output files written by crash_reporter after a Chrome crash.
// In particular, it expects to find a .meta file and a matching .dmp file.
// dirPattern is a glob-style pattern indicating where the crash files should be found.
// FindCrashFilesIn returns an error if the files are not found in the expected
// directory; otherwise, it returns nil.
func FindCrashFilesIn(dirPattern string, files []string) error {
	filePattern := filepath.Join(dirPattern, "chrome*.meta")
	var meta string
	for _, file := range files {
		if match, _ := filepath.Match(filePattern, file); match {
			meta = file
			break
		}
	}

	if meta == "" {
		return errors.Errorf("could not find crash's meta file in %s (possible files: %v)", dirPattern, files)
	}

	dump := strings.TrimSuffix(meta, "meta") + "dmp"
	for _, file := range files {
		if file == dump {
			return nil
		}
	}

	return errors.Errorf("did not find the dmp file %s corresponding to the crash meta file", dump)
}

// getChromePIDs gets the process IDs of all Chrome processes running in the
// system. This will wait for Chrome to be up before returning.
func getChromePIDs(ctx context.Context) ([]int, error) {
	var pids []int
	// Don't just return the list of Chrome PIDs at the moment this is called.
	// Instead, wait for Chrome to be up and then return the pids once it is up
	// and running.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		pids, err = chrome.GetPIDs()
		if err != nil {
			return testing.PollBreak(err)
		}
		if len(pids) == 0 {
			return errors.New("no Chrome processes found")
		}
		return nil
	}, &testing.PollOptions{Timeout: time.Minute}); err != nil {
		return nil, errors.Wrap(err, "failed to get Chrome PIDs")
	}

	return pids, nil
}

// KillAndGetCrashFiles sends SIGSEGV to the root Chrome process, waits for it to
// exit, finds all the new crash files, and then deletes them and returns their paths.
func KillAndGetCrashFiles(ctx context.Context) ([]string, error) {
	dirs, err := cryptohomeCrashDirs(ctx)
	if err != nil {
		return nil, err
	}
	dirs = append(dirs, crash.DefaultDirs()...)
	oldFiles, err := crash.GetCrashes(dirs...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get original crashes")
	}

	pids, err := getChromePIDs(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find Chrome process IDs")
	}

	// Sleep briefly after Chrome starts so it has time to set up breakpad.
	// (Also needed for https://crbug.com/906690)
	const delay = 3 * time.Second
	testing.ContextLogf(ctx, "Sleeping %v to wait for Chrome to stabilize", delay)
	if err := testing.Sleep(ctx, delay); err != nil {
		return nil, errors.Wrap(err, "timed out while waiting for Chrome startup")
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

	newFiles, err := crash.GetCrashes(dirs...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get new crashes")
	}
	newCrashFiles := getNewFiles(oldFiles, newFiles)
	for _, p := range newCrashFiles {
		testing.ContextLog(ctx, "Found expected Chrome crash file ", p)
	}

	// Delete all crash files produced during this test: https://crbug.com/881638
	deleteFiles(ctx, newCrashFiles)

	return newCrashFiles, nil
}
