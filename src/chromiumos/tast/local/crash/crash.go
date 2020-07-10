// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package crash contains utilties common to tests that use crash_reporter and
// crash_sender.
package crash

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/session"
	"chromiumos/tast/local/set"
	"chromiumos/tast/testing"
)

const (
	crashTestInProgressDir = "/run/crash_reporter"
	// crashTestInProgressFile is a special control file that tells crash_reporter
	// to act normally during a crash test. Usually, crash_reporter is being told
	// (by /mnt/stateful_partition/etc/collect_chrome_crashes) to be more
	// aggressive about gathering crash data so that we can debug other, non-
	// crash_reporter tests more easily.
	crashTestInProgressFile = "crash-test-in-progress"
	// anomalyDetectorReadyFile is an indicator that the anomaly detector
	// has started and can detect any new anomalies.
	anomalyDetectorReadyFile = "anomaly-detector-ready"
	// mockConsentFile is a special control file that tells crash_reporter and
	// crash_sender to act as if the user has given consent for crash collection
	// and uploading.
	mockConsentFile = "mock-consent"
	// senderPausePath is the path to the file whose existence indicates that
	// crash_sender should be paused.
	senderPausePath = "/var/lib/crash_sender_paused"
	// senderProcName is the name of the crash_sender process.
	senderProcName = "crash_sender"
	// EarlyCrashDir is the directory where system crashes are stored in absence of persistent storage.
	EarlyCrashDir = "/run/crash_reporter/crash"
	// SystemCrashDir is the directory where system crash reports go.
	SystemCrashDir = "/var/spool/crash"
	// systemCrashStash is a directory to stash pre-existing system crashes during crash tests.
	systemCrashStash = "/var/spool/crash.real"
	// LocalCrashDir is the directory where user crash reports go.
	LocalCrashDir = "/home/chronos/crash"
	// localCrashStash is a directory to stash pre-existing user crashes during crash tests.
	localCrashStash = "/home/chronos/crash.real"
	// UserCrashDir is the directory where crash reports of currently logged in user go.
	UserCrashDir = "/home/chronos/user/crash"
	// userCrashStash is a directory to stash pre-existing crash reports of currently logged in user during crash tests.
	userCrashStash = "/home/chronos/user/crash.real"
	// ClobberCrashDir is a directory where crash reports after an FS clobber go.
	ClobberCrashDir = "/mnt/stateful_partition/reboot_vault/crash"
	// clobberCrashStash is a directory used to stash pre-existing crash reports after an FS clobber. Used in crash tests.
	clobberCrashStash = "/mnt/stateful_partition/reboot_vault/crash.real"
	// userCrashDirs is used for finding the directory name containing a hash for current logged-in user,
	// in order to compare it with crash reporter log.
	userCrashDirs = "/home/chronos/u-*/crash"
	// FilterInPath is the path to the filter-in file.
	FilterInPath = "/run/crash_reporter/filter-in"
	// TestInProgressPath is the path to a file containing the name of the
	// currently-running test, if any.
	TestInProgressPath = "/run/crash_reporter/test-in-prog"

	// BIOSExt is the extension for bios crash files.
	BIOSExt = ".bios_log"
	// CoreExt is the extension for core files.
	CoreExt = ".core"
	// MinidumpExt is the extension for minidump crash files.
	MinidumpExt = ".dmp"
	// LogExt is the extension for log files containing additional information that are written by crash_reporter.
	LogExt = ".log"
	// InfoExt is the extention for info files.
	InfoExt = ".info"
	// ProclogExt is the extention for proclog files.
	ProclogExt = ".proclog"
	// KCrashExt is the extension for log files created by kernel warnings and crashes.
	KCrashExt = ".kcrash"
	// GPUStateExt is the extension for GPU state files written by crash_reporter.
	GPUStateExt = ".i915_error_state.log.xz"
	// MetadataExt is the extension for metadata files written by crash collectors and read by crash_sender.
	MetadataExt = ".meta"
	// CompressedTxtExt is an extension on the compressed log files written by crash_reporter.
	CompressedTxtExt = ".txt.gz"
	// CompressedLogExt is an extension on the compressed log files written by crash_reporter.
	CompressedLogExt = ".log.gz"
	// DevCoredumpExt is an extension for device coredump files.
	DevCoredumpExt = ".devcore"

	// ChromeVerboseConsentFlags provides the flags to enable verbose logging about consent.
	ChromeVerboseConsentFlags = "--vmodule=stats_reporting_controller=1,autotest_private_api=1"

	// FilterInIgnoreAllCrashes is a value to put in the filter-in file if
	// you wish to ignore all crashes that happen during a test.
	FilterInIgnoreAllCrashes = "none"
)

// DefaultDirs returns all standard directories to which crashes are written.
func DefaultDirs() []string {
	return []string{SystemCrashDir, LocalCrashDir, UserCrashDir}
}

// isCrashFile returns true if filename could be the name of a file generated by
// crashes or crash_reporter.
func isCrashFile(filename string) bool {
	knownExts := []string{
		BIOSExt,
		CoreExt,
		MinidumpExt,
		LogExt,
		ProclogExt,
		InfoExt,
		KCrashExt,
		GPUStateExt,
		MetadataExt,
		CompressedTxtExt,
		CompressedLogExt,
		DevCoredumpExt,
	}
	for _, ext := range knownExts {
		if strings.HasSuffix(filename, ext) {
			return true
		}
	}
	return false
}

// GetCrashes returns the paths of all files in dirs generated in response to crashes.
// Nonexistent directories are skipped.
func GetCrashes(dirs ...string) ([]string, error) {
	var crashFiles []string
	for _, dir := range dirs {
		files, err := ioutil.ReadDir(dir)
		if os.IsNotExist(err) {
			continue
		} else if err != nil {
			return nil, err
		}

		for _, fi := range files {
			if isCrashFile(fi.Name()) {
				crashFiles = append(crashFiles, filepath.Join(dir, fi.Name()))
			}
		}
	}
	return crashFiles, nil
}

// GetCrashDir gives the path to the crash directory for given username.
func GetCrashDir(username string) (string, error) {
	if username == "root" || username == "crash" {
		return SystemCrashDir, nil
	}
	p, err := filepath.Glob(userCrashDirs)
	if err != nil {
		// This only happens when userCrashDirs is malformed.
		return "", errors.Wrapf(err, "failed to list up files with pattern [%s]", userCrashDirs)
	}
	if len(p) == 0 {
		return LocalCrashDir, nil
	}
	if len(p) > 1 {
		return "", errors.Errorf("Wrong number of users logged in; got %d, want 1 or 0", len(p))
	}
	return p[0], nil
}

// GetDaemonStoreCrashDirs gives the paths to the daemon store crash directories for the currently active sessions.
func GetDaemonStoreCrashDirs(ctx context.Context) ([]string, error) {
	sessionManager, err := session.NewSessionManager(ctx)
	if err != nil {
		return []string{}, errors.Wrap(err, "couldn't start session manager")
	}

	sessions, err := sessionManager.RetrieveActiveSessions(ctx)
	if err != nil {
		return []string{}, errors.Wrap(err, "couldn't retrieve active sessions")
	}

	var ret []string
	for k := range sessions {
		userhash := sessions[k]
		ret = append(ret, fmt.Sprintf("/home/root/%s/crash", userhash))
	}
	return ret, nil
}

// WaitForCrashFiles waits for each regex in regexes to match a file in dirs that is not also in oldFiles.
// One might use it by
// 1. Getting a list of already-extant files in a directory.
// 2. Doing some operation that will create new files in that directory (e.g. inducing a crash).
// 3. Calling this method to wait for the expected files to appear.
// On success, WaitForCrashFiles returns a map from a regex to a list of files that matched that regex.
// If any regex was not matched, instead returns an error.
//
// When it comes to deleting files, tests should:
//   * Remove matching files that they expect to generate
//   * Leave matching files they do not expect to generate
// If there are more matches than expected and the test can't tell which are expected, it shouldn't delete any.
func WaitForCrashFiles(ctx context.Context, dirs, oldFiles, regexes []string) (map[string][]string, error) {
	var files map[string][]string
	err := testing.Poll(ctx, func(c context.Context) error {
		var newFiles []string
		for _, dir := range dirs {
			dirFiles, err := GetCrashes(dir)
			if err != nil {
				return testing.PollBreak(errors.Wrap(err, "failed to get new crashes"))
			}
			newFiles = append(newFiles, dirFiles...)
		}
		diffFiles := set.DiffStringSlice(newFiles, oldFiles)

		// Reset files each time the poll function is invoked, to avoid
		// repeatedly adding the same file
		files = make(map[string][]string)

		// track regexes that weren't matched.
		var missing []string
		for _, re := range regexes {
			match := false
			for _, f := range diffFiles {
				var err error
				match, err = regexp.MatchString(re, f)
				if err != nil {
					return testing.PollBreak(errors.Wrapf(err, "invalid regexp %s", re))
				}
				if match {
					files[re] = append(files[re], f)
					break
				}
			}
			if !match {
				missing = append(missing, re)
			}
		}
		if len(missing) != 0 {
			return errors.Errorf("no file matched %s (found %s)", strings.Join(missing, ", "), strings.Join(diffFiles, ", "))
		}
		return nil
	}, &testing.PollOptions{Timeout: 15 * time.Second})
	if err != nil {
		return nil, errors.Wrap(err, "timed out while waiting for crash files")
	}
	return files, nil
}

// MoveFilesToOut moves all given files to s.OutDir(). Useful when further
// investigation of some files is needed to debug a test failure.
func MoveFilesToOut(ctx context.Context, outDir string, files ...string) error {
	var firstErr error
	for _, f := range files {
		base := filepath.Base(f)
		testing.ContextLogf(ctx, "Saving %s", base)
		if err := fsutil.MoveFile(f, filepath.Join(outDir, base)); err != nil {
			if firstErr == nil {
				firstErr = errors.Wrapf(err, "couldn't save %s", base)
			}
			testing.ContextLogf(ctx, "Couldn't save %s: %v", base, err)
		}
	}
	return firstErr
}

// RemoveAllFiles removes all files in the values of map.
func RemoveAllFiles(ctx context.Context, files map[string][]string) error {
	var firstErr error
	for _, v := range files {
		for _, f := range v {
			if err := os.Remove(f); err != nil && !os.IsNotExist(err) {
				if firstErr == nil {
					firstErr = errors.Wrapf(err, "couldn't clean up %s", f)
				}
				testing.ContextLogf(ctx, "Couldn't clean up %s: %v", f, err)
			}
		}
	}
	return firstErr
}

// DeleteCoreDumps deletes core dumps whose corresponding minidumps are available.
// It waits for crash_reporter to finish if it is running, in order to avoid
// deleting intermediate core dumps used to generate minidumps. Deleted core
// dumps are logged via ctx.
func DeleteCoreDumps(ctx context.Context) error {
	reporterRunning := func() (bool, error) {
		return processRunning("crash_reporter")
	}
	return deleteCoreDumps(ctx, DefaultDirs(), reporterRunning)
}

func deleteCoreDumps(ctx context.Context, dirs []string, reporterRunning func() (bool, error)) error {
	// First, take a snapshot of core dumps to be deleted.
	paths, size := findCoreDumps(dirs)
	if len(paths) == 0 {
		return nil
	}

	testing.ContextLogf(ctx, "Found %d core dumps (%d bytes)", len(paths), size)

	// Wait for crash_reporter to finish if it is running, in order to avoid
	// deleting intermediate core dumps used to generate minidumps.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		running, err := reporterRunning()
		if err != nil {
			return testing.PollBreak(err)
		}
		if running {
			return errors.New("crash_reporter is still running")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait for crash_reporter to finish")
	}

	// Finally delete core dumps. Note that it is important to use the snapshot
	// taken at the beginning to avoid removing coredumps created by
	// a crash_reporter process started after the wait.
	var firstErr error
	for _, path := range paths {
		if err := os.Remove(path); err != nil {
			testing.ContextLogf(ctx, "Failed to delete %s: %v", path, err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		testing.ContextLog(ctx, "Deleted ", path)
	}
	return firstErr
}

// findCoreDumps returns a list of paths of core dumps whose corresponding
// minidumps are available, and the total size of them.
func findCoreDumps(dirs []string) (paths []string, size int64) {
	const extension = ".core"

	for _, dir := range dirs {
		fis, err := ioutil.ReadDir(dir)
		if err != nil {
			continue
		}

		nameSet := make(map[string]struct{})
		for _, fi := range fis {
			nameSet[fi.Name()] = struct{}{}
		}

		for _, fi := range fis {
			if !strings.HasSuffix(fi.Name(), extension) {
				continue
			}
			dmpName := strings.TrimSuffix(fi.Name(), extension) + ".dmp"
			if _, ok := nameSet[dmpName]; !ok {
				continue
			}
			paths = append(paths, filepath.Join(dir, fi.Name()))
			size += fi.Size()
		}
	}

	sort.Strings(paths)
	return paths, size
}

// processRunning checks if a process named procName is running.
func processRunning(procName string) (bool, error) {
	ps, err := process.Processes()
	if err != nil {
		return false, err
	}
	for _, p := range ps {
		n, err := p.Name()
		if err != nil {
			continue
		}
		if n == procName {
			return true, nil
		}
	}
	return false, nil
}

// MarkTestInProgress writes |name| to |TestInProgressPath|, indicating to crash_reporter
// that the given test is in progress.
func MarkTestInProgress(name string) error {
	if err := ioutil.WriteFile(TestInProgressPath, []byte(name), 0644); err != nil {
		return errors.Wrap(err, "failed to write in-progress test name: ")
	}
	return nil
}

// MarkTestDone removes the file indicating which test is running.
func MarkTestDone() error {
	if err := os.Remove(TestInProgressPath); err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, "failed to remove in-progress test name: ")
	}
	return nil
}
