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
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/process"

	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/session"
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
	// FilterOutPath is the path to the filter-out file.
	FilterOutPath = "/run/crash_reporter/filter-out"
	// testInProgressPath is the path to a file containing the name of the
	// currently-running test, if any.
	testInProgressPath = "/run/crash_reporter/test-in-prog"

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
	DevCoredumpExt = ".devcore.gz"
	// ECCrashExt is an extension for ec crash dumps
	ECCrashExt = ".eccrash"
	// JavaScriptStackExt is the extension for JavaScript stacks.
	JavaScriptStackExt = ".js_stack"

	// ChromeVerboseConsentFlags provides the flags to enable verbose logging about consent.
	ChromeVerboseConsentFlags = "--vmodule=stats_reporting_controller=1,autotest_private_api=1"

	// FilterInIgnoreAllCrashes is a value to put in the filter-in file if
	// you wish to ignore all crashes that happen during a test.
	FilterInIgnoreAllCrashes = "none"
)

var (
	markTestInProgressVar = testing.RegisterVarString(
		"crash.markTestInProgress", // The variable controls if the test-in-prog file will be created.
		// Default value is true, create file test-in-prog by default.
		// When set the var to "false", test-in-prog file will not be created.
		"true",
		"The variable that controls if test-in-prog file will be created")

	testInProgressPrefixVar = testing.RegisterVarString(
		"crash.testInProgressPrefix", // The variable prefixed to the test case name in test-in-prog file.
		"",                           // By default no prefix will be added.
		"The string that will be prefixed to the test case name in test-in-prog file")
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
		ECCrashExt,
		JavaScriptStackExt,
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
func GetCrashDir(ctx context.Context, username string) (string, error) {
	d, err := GetDaemonStoreCrashDirs(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to get daemon store directories")
	}
	if len(d) == 0 {
		if username == "root" || username == "crash" {
			return SystemCrashDir, nil
		}
		return LocalCrashDir, nil
	}
	if len(d) > 1 {
		return "", errors.Errorf("Wrong number of users logged in; got %d, want 1 or 0", len(d))
	}
	return d[0], nil
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

// GetDaemonStoreConsentDirs gives the paths to the daemon store consent directories for the currently active sessions.
func GetDaemonStoreConsentDirs(ctx context.Context) ([]string, error) {
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
		ret = append(ret, fmt.Sprintf("/home/root/%s/uma-consent", userhash))
	}
	// If no one is logged in, that's okay -- just return an empty list and don't fail.
	// (Many tests are run when no user is logged in.)
	return ret, nil
}

// RegexesNotFound is an error type, used to indicate that
// WaitForCrashFiles didn't find matches for all of the regexs.
type RegexesNotFound struct {
	// Missing lists all the regexs that weren't matched.
	Missing []string
	// Files lists all the files that were checked against the regexes.
	Files []string
	// PartialMatches gives all the regexes that were matched and the files that
	// matched them.
	PartialMatches map[string][]string
	// Dirs lists all directories where files are searched.
	Dirs []string
}

// Error returns a string describing the error. The classic Error function for
// the error interface.
func (e RegexesNotFound) Error() string {
	return fmt.Sprintf("timed out while waiting for crash files: no file matched %s (dirs %s) (found %s)",
		strings.Join(e.Missing, ", "), strings.Join(e.Dirs, ", "), strings.Join(e.Files, ", "))
}

// waitForCrashFilesOptions is a list of options for the WaitForCrashFiles
// function. External users can manipulate via the WaitForCrashFilesOpt-returning
// functions below.
type waitForCrashFilesOptions struct {
	timeout         time.Duration
	optionalRegexes []string
}

// WaitForCrashFilesOpt is a self-referential function can be used to configure WaitForCrashFiles.
// See https://commandcenter.blogspot.com.au/2014/01/self-referential-functions-and-design.html
// for details about this pattern.
type WaitForCrashFilesOpt func(w *waitForCrashFilesOptions)

// Timeout returns a WaitForCrashFilesOpts which will set the timeout of WaitForCrashFiles
// to the indicated duration.
func Timeout(timeout time.Duration) WaitForCrashFilesOpt {
	return func(w *waitForCrashFilesOptions) {
		w.timeout = timeout
	}
}

// OptionalRegexes instructs WaitForCrashFiles to look for files matching the
// given regexes and return those as normal in the return map. However, if
// the optional regexes are not matched, the polling loop will still exit and
// WaitForCrashFiles will not return an error.
func OptionalRegexes(optionalRegexes []string) WaitForCrashFilesOpt {
	return func(w *waitForCrashFilesOptions) {
		w.optionalRegexes = optionalRegexes
	}
}

// WaitForCrashFiles waits for each regex in regexes to match a file in dirs.
// The directory is not matched against the regex, and the regex must match the
// entire filename. (So  /var/spool/crash/hello_world.20200331.1234.log will NOT
// match 'world\.\d{1,8}\.\d{1,8}\.log'.)
// One might use it by
// 1. Doing some operation that will create new files in that directory (e.g. inducing a crash).
// 2. Calling this method to wait for the expected files to appear.
// On success, WaitForCrashFiles returns a map from a regex to a list of files that matched that regex.
// If any regex was not matched, instead returns an error of type RegexesNotFound.
//
// When it comes to deleting files, tests should:
//   * Remove matching files that they expect to generate
//   * Leave matching files they do not expect to generate
// If there are more matches than expected and the test can't tell which are expected, it shouldn't delete any.
func WaitForCrashFiles(ctx context.Context, dirs, regexes []string, opts ...WaitForCrashFilesOpt) (map[string][]string, error) {
	w := &waitForCrashFilesOptions{timeout: 15 * time.Second}
	for _, opt := range opts {
		opt(w)
	}

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

		// Reset files each time the poll function is invoked, to avoid
		// repeatedly adding the same file
		files = make(map[string][]string)

		// track regexes that weren't matched.
		var missing []string
		for _, rx := range []struct {
			regexp   []string
			optional bool
		}{
			{regexes, false},
			{w.optionalRegexes, true},
		} {
			for _, re := range rx.regexp {
				match := false
				for _, f := range newFiles {
					base := filepath.Base(f)
					matchThisFile, err := regexp.MatchString("^"+re, base)
					if err != nil {
						return testing.PollBreak(errors.Wrapf(err, "invalid regexp %s", re))
					}
					if matchThisFile {
						// Wait for meta files to have "done=1".
						if strings.HasSuffix(f, ".meta") {
							var contents []byte
							if contents, err = ioutil.ReadFile(f); err != nil {
								// There's a known issue with cryptohome 'flickering'
								// occasionally. (b/189707927) If one process writes a file, a
								// different process trying to read it the instant the file
								// shows up may not be able to. So don't testing.PollBreak here,
								// just retry and see if we can read on the next go-round.
								return errors.Wrap(err, "failed to read .meta file")
							}
							if !strings.Contains(string(contents), "done=1") {
								// Not there yet.
								matchThisFile = false
							}
						}
					}
					if matchThisFile {
						files[re] = append(files[re], f)
						match = true
					}
				}
				if !match && !rx.optional {
					missing = append(missing, re)
				}
			}
		}
		if len(missing) != 0 {
			return &RegexesNotFound{Missing: missing, Files: newFiles, PartialMatches: files, Dirs: dirs}
		}
		return nil
	}, &testing.PollOptions{Timeout: w.timeout})
	if err != nil {
		var regexesNotFoundError *RegexesNotFound
		if errors.As(err, &regexesNotFoundError) {
			// Return unwrapped error, since we promise to return a RegexesNotFound
			// error, not an error that wraps a RegexesNotFound error. testing.Poll
			// will run errors.Wrap on the error returned from the lambda.
			return nil, *regexesNotFoundError
		}

		return nil, errors.Wrap(err, "unable to find crash files")
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
	const zombieStatus = "Z"

	ps, err := process.Processes()
	if err != nil {
		return false, err
	}
	for _, p := range ps {
		status, err := p.Status()
		if err != nil {
			continue
		}

		n, err := p.Name()
		if err != nil {
			continue
		}
		if n == procName && status[0] != zombieStatus {
			return true, nil
		}
	}
	return false, nil
}

// shouldMarkTestInProgress parses markTestInProgressVar to a boolean value.
// If the value of markTestInProgressVar is not supported by strconv.ParseBool(), it will return true.
func shouldMarkTestInProgress(ctx context.Context) bool {
	markTestInProgress, err := strconv.ParseBool(markTestInProgressVar.Value())

	//If any parse error happens, set the value to true.
	if err != nil {
		testing.ContextLogf(ctx, "Failed to parse crash.markTestInProgress value %q, use default value true", markTestInProgressVar.Value())
		markTestInProgress = true
	}
	return markTestInProgress
}

// MarkTestInProgress writes |name| to |testInProgressPath|, indicating to crash_reporter
// that the given test is in progress.
func MarkTestInProgress(ctx context.Context, name string) error {
	if !shouldMarkTestInProgress(ctx) {
		return nil
	}
	if err := ioutil.WriteFile(testInProgressPath, []byte(testInProgressPrefixVar.Value()+name), 0644); err != nil {
		return errors.Wrap(err, "failed to write in-progress test name")
	}
	return nil
}

// MarkTestDone removes the file indicating which test is running.
func MarkTestDone(ctx context.Context) error {
	// Don't remove the test-in-prog file if it's not created by tast in MarkTestInProgress function.
	if !shouldMarkTestInProgress(ctx) {
		return nil
	}
	if err := os.Remove(testInProgressPath); err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, "failed to remove in-progress test name")
	}
	return nil
}
