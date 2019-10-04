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
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/crash"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	localCrash "chromiumos/tast/local/crash"
	"chromiumos/tast/local/set"
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

	// VModuleFlag is passed to Chrome when testing Chrome crashes. It allows us
	// to debug certain failures, particularly cases where consent didn't get set
	// up correctly, as well as any problems with the upcoming crashpad changeover.
	VModuleFlag = "--vmodule=chrome_crash_reporter_client=1,breakpad_linux=1,crashpad=1,crashpad_linux=1"
)

// ProcessType is an enum listed the types of Chrome processes we can kill.
type ProcessType int

const (
	// Browser indicates the root Chrome process, the one without a --type flag.
	Browser ProcessType = iota
	// GPUProcess indicates a process with --type=gpu-process. We use GPUProcess
	// to stand in for most types of non-Browser processes, including renderer and
	// zygote; given the comments above Chrome's NonBrowserCrashHandler class, the
	// code path should be similar enough that we don't need separate tests for
	// those process types.
	GPUProcess
	// Broker indicates a process with --type=broker. Broker processes go through
	// a special code path because they are forked directly.
	Broker
)

// String returns a string naming the given ProcessType, suitable for displaying
// in log and error messages.
func (ptype ProcessType) String() string {
	return [...]string{"Browser", "GPUProcess", "Broker"}[ptype]
}

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

// getNonBrowserProcess returns the id of a single Chrome process of the
// indicated type. If more than one such process exists, the first one is
// returned. Does not wait for the process to come up.
func getNonBrowserProcess(ctx context.Context, ptype ProcessType) (pid int, err error) {
	var processes []process.Process
	switch ptype {
	case GPUProcess:
		processes, err = chrome.GetGPUProcesses()
	case Broker:
		processes, err = chrome.GetBrokerProcesses()
	default:
		return -1, errors.Errorf("unknown ProcessType %s", ptype)
	}
	if err != nil {
		return -1, errors.Wrapf(err, "error looking for %s process", ptype)
	}
	if len(processes) == 0 {
		return -1, errors.Errorf("no Chrome %s processes found", ptype)
	}
	return int(processes[0].Pid), nil
}

// waitForMetaFile waits for a .meta file corresponding to the given pid to appear
// in one of the directories. Any file that matches a name in oldFiles is ignored.
// Return nil if the file is found.
func waitForMetaFile(ctx context.Context, pid int, dirs, oldFiles []string) error {
	ending := fmt.Sprintf(".*\\.%d\\.meta", pid)
	_, err := localCrash.WaitForCrashFiles(ctx, dirs, oldFiles, []string{ending})
	if err != nil {
		return errors.Wrap(err, "error waiting for .meta file")
	}
	return nil
}

// killNonBrowser implements KillAndGetCrashFiles for any process type OTHER
// than the root Browser type.
func killNonBrowser(ctx context.Context, ptype ProcessType, dirs, oldFiles []string) error {
	testing.ContextLogf(ctx, "Hunting for a %s process", ptype)
	var toKill int
	// It's possible that the root browser process just started and hasn't created
	// the GPU or broker process yet. Also, if the target process disappears during
	// the 3 second stabilization period, we'd be willing to try a different one.
	// Retry until we manage to successfully send a SEGV.
	err := testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		toKill, err = getNonBrowserProcess(ctx, ptype)
		if err != nil {
			return errors.Wrapf(err, "could not find %s process to kill", ptype)
		}

		// Sleep briefly after the Chrome process we want starts so it has time to set
		// up breakpad.
		const delay = 3 * time.Second
		testing.ContextLogf(ctx, "Sleeping %v to wait for Chrome to stabilize", delay)
		if err := testing.Sleep(ctx, delay); err != nil {
			return testing.PollBreak(errors.Wrap(err, "timed out while waiting for Chrome startup"))
		}

		testing.ContextLogf(ctx, "Sending SIGSEGV to target Chrome %s process %d", ptype, toKill)
		if err = syscall.Kill(toKill, syscall.SIGSEGV); err != nil {
			if errno, ok := err.(syscall.Errno); ok && errno == syscall.ESRCH {
				return errors.Errorf("target process %d does not exist", toKill)
			}
			return testing.PollBreak(errors.Wrapf(err, "could not kill target process %d", toKill))
		}
		return nil
	}, nil)
	if err != nil {
		return errors.Wrapf(err, "failed to find & kill a %s process", ptype)
	}

	// We don't wait for the process to exit here. Most non-Browser processes
	// don't actually exit when sent a SIGSEGV from outside the process. (This
	// is the expected behavior -- see
	// https://groups.google.com/a/chromium.org/d/msg/chromium-dev/W_vGBMHxZFQ/wPkqHBgBAgAJ)
	// Instead, we wait for the crash meta file to be written out.
	testing.ContextLog(ctx, "Waiting for meta file to appear")
	if err = waitForMetaFile(ctx, toKill, dirs, oldFiles); err != nil {
		return errors.Wrap(err, "failed waiting for target to write crash files")
	}
	return nil
}

// killBrowser implements the specialized logic for killing & waiting for the
// root Browser process. The principle difference between this and killNonBrowser
// is that this waits for the SEGV'ed process to die instead of waiting for a
// .meta file. Why? First, because the ChromeCrash....Direct tests don't create
// .meta files  -- they just create .dmp files with more-difficult-to-determine
// names -- and ChromeCrashLoop doesn't create files at all on one of its kills.
// Second, we really want the Browser process we kill here to exit before
// ChromeCrash[Not]LoggedIn's loop tries to kill a non-Browser process; we don't
// want the non-Browser kills to pick up an orphaned process, because they won't
// create crash output correctly.
func killBrowser(ctx context.Context) error {
	pids, err := getChromePIDs(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to find Chrome process IDs")
	}

	// Sleep briefly after Chrome starts so it has time to set up breakpad.
	// (Also needed for https://crbug.com/906690)
	const delay = 3 * time.Second
	testing.ContextLogf(ctx, "Sleeping %v to wait for Chrome to stabilize", delay)
	if err := testing.Sleep(ctx, delay); err != nil {
		return errors.Wrap(err, "timed out while waiting for Chrome startup")
	}

	// The root Chrome process (i.e. the one that doesn't have another Chrome process
	// as its parent) is the browser process. It's not sandboxed, so it should be able
	// to write a minidump file when it crashes.
	rp, err := chrome.GetRootPID()
	if err != nil {
		return errors.Wrap(err, "failed to get root Chrome PID")
	}
	testing.ContextLog(ctx, "Sending SIGSEGV to root Chrome process ", rp)
	if err = syscall.Kill(rp, syscall.SIGSEGV); err != nil {
		return errors.Wrap(err, "failed to kill process")
	}

	// Wait for all the processes to die (not just the root one). This avoids
	// messing up other killNonBrowser tests that might try to kill an orphaned
	// process.
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
		return errors.Wrap(err, "Chrome didn't exit")
	}
	testing.ContextLog(ctx, "All Chrome processes exited")
	return nil
}

// KillAndGetCrashFiles sends SIGSEGV to the given Chrome process, waits for it to
// crash, finds all the new crash files, and then deletes them and returns their paths.
func KillAndGetCrashFiles(ctx context.Context, ptype ProcessType) ([]string, error) {
	dirs, err := cryptohomeCrashDirs(ctx)
	if err != nil {
		return nil, err
	}
	dirs = append(dirs, crash.DefaultDirs()...)
	oldFiles, err := crash.GetCrashes(dirs...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get original crashes")
	}

	if ptype == Browser {
		if err = killBrowser(ctx); err != nil {
			return nil, errors.Wrap(err, "failed to kill Browser process")
		}
	} else {
		if err = killNonBrowser(ctx, ptype, dirs, oldFiles); err != nil {
			return nil, errors.Wrapf(err, "failed to kill %s process", ptype.String())
		}
	}

	newFiles, err := crash.GetCrashes(dirs...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get new crashes")
	}
	newCrashFiles := set.DiffStringSlice(newFiles, oldFiles)
	for _, p := range newCrashFiles {
		testing.ContextLog(ctx, "Found expected Chrome crash file ", p)
	}

	// Delete all crash files produced during this test: https://crbug.com/881638
	deleteFiles(ctx, newCrashFiles)

	return newCrashFiles, nil
}
