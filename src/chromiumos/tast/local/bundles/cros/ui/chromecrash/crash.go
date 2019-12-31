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
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crash"
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
	VModuleFlag = "--vmodule=chrome_crash_reporter_client=1,breakpad_linux=1,crashpad=1,crashpad_linux=1,broker_process=3,sandbox_linux=3"

	// nanosecondsPerMillisecond helps convert ns to ms. Needed to deal with
	// gopsutil/process which reports creation times in milliseconds-since-UNIX-epoch,
	// while golang's time can only be constructed using nanoseconds-since-UNIX-epoch.
	nanosecondsPerMillisecond = 1000 * 1000
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
	switch ptype {
	case Browser:
		return "Browser"
	case GPUProcess:
		return "GPUProcess"
	case Broker:
		return "Broker"
	default:
		return "Unknown ProcessType " + strconv.Itoa(int(ptype))
	}
}

// CrashFileType is an enum listing the types of crash output files the crash
// system might produce.
type CrashFileType int

const (
	// MetaFile refers to the .meta files created by crash_reporter. This is the
	// normal crash file type.
	MetaFile CrashFileType = iota
	// BreakpadDmp indicates the .dmp files generated directly by breakpad and
	// crashpad. We only see these when we are skipping crash_reporter and having
	// breakpad / crashpad dump directly.
	BreakpadDmp
)

// String returns a string naming the given CrashFileType, suitable for displaying
// in log and error messages.
func (cfType CrashFileType) String() string {
	switch cfType {
	case MetaFile:
		return "MetaFile"
	case BreakpadDmp:
		return "BreakpadDmp"
	default:
		return "Unknown CrashFileType " + strconv.Itoa(int(cfType))
	}
}

// CrashTester maintains state between different parts of the Chrome crash tests.
// It should be created (via New) before chrome.New is called. Close should be
// called at the end of the test.
type CrashTester struct {
	ptype   ProcessType
	waitFor CrashFileType
}

// New returns a CrashTester. This must be called before chrome.New.
func New(ptype ProcessType, waitFor CrashFileType) (*CrashTester, error) {
	return &CrashTester{
		ptype:   ptype,
		waitFor: waitFor,
	}, nil
}

// Close closes a CrashTester. It must be called on all CrashTesters returned from New.
func (ct *CrashTester) Close() {

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

// anyPIDsExist returns true if any PIDs in pids are still present. To avoid
// PID races, only processes created before the indicated time are considered.
func anyPIDsExist(pids []int, createdBefore time.Time) bool {
	createdBeforeMS := createdBefore.UnixNano() / nanosecondsPerMillisecond
	for _, pid := range pids {
		proc, err := process.NewProcess(int32(pid))
		if err != nil {
			// Assume process exited.
			continue
		}
		// If there are errors, again, assume the process exited.
		if createTimeMS, err := proc.CreateTime(); err == nil && createTimeMS <= createdBeforeMS {
			return true
		}
	}
	return false
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

// getNonBrowserProcess returns a Process structure of a single Chrome process
// of the indicated type. If more than one such process exists, the first one is
// returned. Does not wait for the process to come up -- if none exist, this
// just returns an error.
func (ct *CrashTester) getNonBrowserProcess(ctx context.Context) (process.Process, error) {
	var processes []process.Process
	var err error
	switch ct.ptype {
	case GPUProcess:
		processes, err = chrome.GetGPUProcesses()
	case Broker:
		processes, err = chrome.GetBrokerProcesses()
	default:
		return process.Process{}, errors.Errorf("unknown ProcessType %s", ct.ptype)
	}
	if err != nil {
		return process.Process{}, errors.Wrapf(err, "error looking for Chrome %s", ct.ptype)
	}
	if len(processes) == 0 {
		return process.Process{}, errors.Errorf("no Chrome %s's found", ct.ptype)
	}
	return processes[0], nil
}

// waitForMetaFile waits for a .meta file corresponding to the given pid to appear
// in one of the directories. Any file that matches a name in oldFiles is ignored.
// Return nil if the file is found.
func waitForMetaFile(ctx context.Context, pid int, dirs, oldFiles []string) error {
	ending := fmt.Sprintf(`.*\.%d\.meta`, pid)
	_, err := crash.WaitForCrashFiles(ctx, dirs, oldFiles, []string{ending})
	if err != nil {
		return errors.Wrap(err, "error waiting for .meta file")
	}
	return nil
}

// waitForBreakpadDmpFile waits for a .dmp file corresponding to the given pid
// to appear in one of the directories. Any file that matches a name in oldFiles
// is ignored. Return nil if the file is found.
func waitForBreakpadDmpFile(ctx context.Context, pid int, dirs, oldFiles []string) error {
	const fileName = `chromium-.*-minidump-.*\.dmp`
	err := testing.Poll(ctx, func(c context.Context) error {
		files, err := crash.WaitForCrashFiles(ctx, dirs, oldFiles, []string{fileName})
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "error waiting for .dmp file"))
		}

		// We don't want to immediately fail if we can't parse a .dmp file. .dmp files
		// may be in the middle of being written out when this code runs, in which
		// case we'll get all sorts of odd errors. There's also a (small) possibility
		// of one malformed .dmp file (maybe from crash_reporter) matching, in
		// which case we just want to look at the other .dmp files. Remember any
		// errors for later debugging, but don't stop looking and, above all,
		// don't testing.PollBreak.
		errorList := make([]error, 0)
		for _, fileName := range files {
			if found, err := crash.IsBreakpadDmpFileForPID(fileName, pid); err != nil {
				errorList = append(errorList, errors.Wrap(err, "error scanning "+fileName))
			} else if found {
				// Success, ignore all other errors.
				return nil
			}
		}
		if len(errorList) == 0 {
			return errors.Errorf("could not find dmp file with PID %d in %v", pid, files)
		}
		if len(errorList) == 1 {
			return errorList[0]
		}
		return errors.Errorf("multiple errors found scanning .dmp files: %v", errorList)
	}, nil)
	if err != nil {
		return errors.Wrap(err, "error waiting for .dmp file")
	}
	return nil
}

// killNonBrowser implements the killing heart of KillAndGetCrashFiles for any
// process type OTHER than the root Browser type.
func (ct *CrashTester) killNonBrowser(ctx context.Context, dirs, oldFiles []string) error {
	testing.ContextLogf(ctx, "Hunting for a %s", ct.ptype)
	var toKill process.Process
	// It's possible that the root browser process just started and hasn't created
	// the GPU or broker process yet. Also, if the target process disappears during
	// the 3 second stabilization period, we'd be willing to try a different one.
	// Retry until we manage to successfully send a SEGV.
	err := testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		toKill, err = ct.getNonBrowserProcess(ctx)
		if err != nil {
			return errors.Wrapf(err, "could not find Chrome %s to kill", ct.ptype)
		}

		// Sleep briefly after the Chrome process we want starts so it has time to set
		// up breakpad.
		const delay = 3 * time.Second
		createTimeMS, err := toKill.CreateTime()
		if err != nil {
			return errors.Wrap(err, "could not get create time of process")
		}
		createTime := time.Unix(0, createTimeMS*nanosecondsPerMillisecond)
		timeToSleep := delay - time.Since(createTime)
		if timeToSleep > 0 {
			testing.ContextLogf(ctx, "Sleeping %v to wait for Chrome to stabilize", timeToSleep)
			if err := testing.Sleep(ctx, timeToSleep); err != nil {
				return testing.PollBreak(errors.Wrap(err, "timed out while waiting for Chrome startup"))
			}
		}

		testing.ContextLogf(ctx, "Sending SIGSEGV to target Chrome %s pid %d", ct.ptype, toKill.Pid)
		if err = syscall.Kill(int(toKill.Pid), syscall.SIGSEGV); err != nil {
			if errno, ok := err.(syscall.Errno); ok && errno == syscall.ESRCH {
				return errors.Errorf("target process %d does not exist", toKill.Pid)
			}
			return testing.PollBreak(errors.Wrapf(err, "could not kill target process %d", toKill.Pid))
		}
		return nil
	}, nil)
	if err != nil {
		return errors.Wrapf(err, "failed to find & kill a Chrome %s", ct.ptype)
	}

	// We don't wait for the process to exit here. Most non-Browser processes
	// don't actually exit when sent a SIGSEGV from outside the process. (This
	// is the expected behavior -- see
	// https://groups.google.com/a/chromium.org/d/msg/chromium-dev/W_vGBMHxZFQ/wPkqHBgBAgAJ)
	// Instead, we wait for the crash file to be written out.
	switch ct.waitFor {
	case MetaFile:
		testing.ContextLog(ctx, "Waiting for meta file to appear")
		if err = waitForMetaFile(ctx, int(toKill.Pid), dirs, oldFiles); err != nil {
			return errors.Wrap(err, "failed waiting for target to write crash meta files")
		}
	case BreakpadDmp:
		testing.ContextLog(ctx, "Waiting for dmp file to appear")
		if err = waitForBreakpadDmpFile(ctx, int(toKill.Pid), dirs, oldFiles); err != nil {
			return errors.Wrap(err, "failed waiting for target to write crash dmp files")
		}
	default:
		return errors.New("unknown CrashFileType " + string(ct.waitFor))
	}

	return nil
}

// killBrowser implements the specialized logic for killing & waiting for the
// root Browser process. The principle difference between this and killNonBrowser
// is that this waits for the SEGV'ed process to die instead of waiting for a
// .meta file. We can't wait for a file to be created because ChromeCrashLoop
// doesn't create files at all on one of its kills.
func killBrowser(ctx context.Context) error {
	pids, err := getChromePIDs(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to find Chrome process IDs")
	}
	preSleepTime := time.Now()

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
		if anyPIDsExist(pids, preSleepTime) {
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
func (ct *CrashTester) KillAndGetCrashFiles(ctx context.Context) ([]string, error) {
	dirs, err := cryptohomeCrashDirs(ctx)
	if err != nil {
		return nil, err
	}
	dirs = append(dirs, crash.DefaultDirs()...)
	oldFiles, err := crash.GetCrashes(dirs...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get original crashes")
	}

	if ct.ptype == Browser {
		if err = killBrowser(ctx); err != nil {
			return nil, errors.Wrap(err, "failed to kill Browser process")
		}
	} else {
		if err = ct.killNonBrowser(ctx, dirs, oldFiles); err != nil {
			return nil, errors.Wrapf(err, "failed to kill Chrome %s", ct.ptype)
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
