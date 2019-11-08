// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package crash contains functionality shared by tests that exercise the crash reporter.
package crash

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/metrics"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const (
	// CorePattern is the full path of the core pattern file.
	CorePattern = "/proc/sys/kernel/core_pattern"

	// TestCert is a certificate for generating consent to log crash info.
	TestCert = "testcert.p12"

	// CrashReporterPath is the full path of the crash reporter binary.
	CrashReporterPath = "/sbin/crash_reporter"

	// CrashReporterEnabledPath is the full path for crash handling data file.
	CrashReporterEnabledPath = "/var/lib/crash_reporter/crash-handling-enabled"

	crashTestInProgress    = "/run/crash_reporter/crash-test-in-progress"
	crasherPath            = "/usr/local/libexec/tast/helpers/local/cros/platform.UserCrash.crasher"
	crashReporterLogFormat = "[user] Received crash notification for %s[%d] sig 11, user %s group %s (%s)"
	crashSenderPath        = "/sbin/crash_sender"
	crashSenderRateDir     = "/var/lib/crash_sender"
	pauseFile              = "/var/lib/crash_sender_paused"
	systemCrashDir         = "/var/spool/crash"
	fallbackUserCrashDir   = "/home/chronos/crash"
	userCrashDirs          = "/home/chronos/u-*/crash"
	messagesFile           = "/var/log/messages"
	crashRunStateDir       = "/run/crash_reporter"
	mockCrashSending       = crashRunStateDir + "/mock-crash-sending"
)

var pidRegex = regexp.MustCompile(`(?m)^pid=(\d+)$`)
var userCrashDirRegex = regexp.MustCompile("/home/chronos/u-([a-f0-9]+)/crash")
var nonAlphaNumericRegex = regexp.MustCompile("[^0-9A-Za-z]")

// CrasherOptions stores configurations for running crasher process.
type CrasherOptions struct {
	Username   string
	CauseCrash bool
	Consent    bool
}

// CrasherResult stores result status and outputs from a crasher process execution.
type CrasherResult struct {
	// ReturnCode is the return code of the crasher process.
	ReturnCode int

	// Crashed stores whether the crasher returned segv error code.
	Crashed bool

	// CrashReporterCaught stores whether the crash reporter caught a segv.
	CrashReporterCaught bool

	// Minidump (.dmp) crash report filename.
	Minidump string

	// Basename of the crash report files.
	Basename string

	// .meta crash report filename.
	Meta string

	// .log crash report filename.
	Log string
}

// DefaultCrasherOptions creates a CrasherOptions which actually cause and catch crash.
// Username is not populated as it should be set explicitly by each test.
func DefaultCrasherOptions() CrasherOptions {
	return CrasherOptions{
		CauseCrash: true,
		Consent:    true,
	}
}

// exitCode extracts exit code from error returned by exec.Command.Run().
// Equivalent to this in Go version >= 1.12: (*cmd.ProcessState).ExitCode()
// This will return code for backward compatibility.
// TODO(yamaguchi): Remove this after golang is uprevved to >= 1.12.
func exitCode(cmdErr error) (int, error) {
	e, ok := cmdErr.(*exec.ExitError)
	if !ok {
		return 0, errors.Errorf("failed to cast to exec.ExitError err=%v", cmdErr)
	}
	s, ok := e.Sys().(syscall.WaitStatus)
	if !ok {
		return 0, errors.Errorf("failed to cast to syscall.WaitStatus err=%v", cmdErr)
	}
	if s.Exited() {
		return s.ExitStatus(), nil
	}
	if !s.Signaled() {
		return 0, errors.Errorf("unexpected exit status: status=%v", s)
	}
	return -int(s.Signal()), nil
}

func checkCrashDirectoryPermissions(path string) error {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return err
	}
	s, ok := fileInfo.Sys().(*syscall.Stat_t)
	if !ok {
		return errors.New("failed to cast to Stat_t")
	}
	usr, err := user.LookupId(strconv.FormatInt(int64(s.Uid), 10))
	if err != nil {
		return err
	}
	grp, err := user.LookupGroupId(strconv.FormatInt(int64(s.Gid), 10))
	if err != nil {
		return err
	}
	mode := fileInfo.Mode()

	permittedModes := make(map[os.FileMode]struct{})
	var expectedUser string
	var expectedGroup string
	if strings.HasPrefix(path, "/var/spool/crash") {
		if fileInfo.IsDir() {
			files, err := ioutil.ReadDir(path)
			if err != nil {
				return errors.Wrapf(err, "failed to read directory %s", path)
			}
			for _, f := range files {
				if err := checkCrashDirectoryPermissions(filepath.Join(path, f.Name())); err != nil {
					return err
				}
			}
			permittedModes[os.FileMode(0770)|os.ModeDir|os.ModeSetgid] = struct{}{}
		} else {
			permittedModes[os.FileMode(0660)] = struct{}{}
			permittedModes[os.FileMode(0640)] = struct{}{}
			permittedModes[os.FileMode(0644)] = struct{}{}
		}
		expectedUser = "root"
		expectedGroup = "crash-access"
	} else {
		permittedModes[os.ModeDir|os.FileMode(0700)] = struct{}{}
		expectedUser = "chronos"
		expectedGroup = "chronos"
	}
	if usr.Username != expectedUser || grp.Name != expectedGroup {
		return errors.Errorf("ownership of %s got %s.%s; want %s.%s",
			path, usr.Username, grp.Name, expectedUser, expectedGroup)
	}
	if _, found := permittedModes[mode]; !found {
		var keys []os.FileMode
		for k := range permittedModes {
			keys = append(keys, k)
		}
		return errors.Errorf("mode of %s got %v; want either of %v", path, mode, keys)
	}
	return nil
}

func getCrashDir(username string) (string, error) {
	if username == "root" || username == "crash" {
		return systemCrashDir, nil
	}
	p, err := filepath.Glob(userCrashDirs)
	if err != nil {
		// This only happens when userCrashDirs is malformed.
		return "", errors.Wrapf(err, "failed to list up files with pattern [%s]", userCrashDirs)
	}
	if len(p) == 0 {
		return fallbackUserCrashDir, nil
	}
	return p[0], nil
}

// canonicalizeCrashDir converts /home/chronos crash directory to /home/user counterpart.
func canonicalizeCrashDir(path string) string {
	m := userCrashDirRegex.FindStringSubmatch(path)
	if m == nil {
		return path
	}
	return filepath.Join("/home", m[1], "crash")
}

// enableSystemSending allows to run system crash_sender.
func enableSystemSending() error {
	if err := os.Remove(pauseFile); err != nil && !os.IsNotExist(err) {
		return errors.Wrapf(err, "failed to remove pause file %s", pauseFile)
	}
	return nil
}

// disableSystemSending disallows to run system crash_sender.
func disableSystemSending() error {
	if f, err := os.Stat(pauseFile); err != nil {
		if !os.IsNotExist(err) {
			return errors.Wrapf(err, "failed to stat %s", pauseFile)
		}
		// Create policy file that enables metrics/consent.
		if err := ioutil.WriteFile(pauseFile, []byte{}, 0644); err != nil {
			return errors.Wrap(err, "failed to create pause file")
		}
	} else {
		if !f.Mode().IsRegular() {
			return errors.Errorf("%s was not a regular file", pauseFile)
		}
	}
	return nil
}

// setCrashTestInProgress creates a file to tell crash_repoter that a crash_repoter test is in progress.
func setCrashTestInProgress() error {
	if err := ioutil.WriteFile(crashTestInProgress, []byte("in-progress"), 0644); err != nil {
		return errors.Wrapf(err, "failed writing in-progress state file %s", crashTestInProgress)
	}
	return nil
}

// unsetCrashTestInProgress tells crash_repoter that no crash_repoter test is in progress.
func unsetCrashTestInProgress() error {
	if err := os.Remove(crashTestInProgress); err != nil && !os.IsNotExist(err) {
		return errors.Wrapf(err, "failed to remove in-progress state file %s", crashTestInProgress)
	}
	return nil
}

// stashCrashFiles moves contents of crash directory to a temporary backup directory.
// Those files can be restored later by calling the function returned by this function.
// Doesn't support recursive stashing.
func stashCrashFiles(userName string) (func() error, error) {
	crashDir, err := getCrashDir(userName)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get crash dir for user %s", userName)
	}
	// Move to subdirectory that shares parent dir with the original, which should be in the same filesystem.
	parent := filepath.Dir(crashDir)
	tempDir, err := ioutil.TempDir(parent, "tast_unittest_crash.")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create temporary directory under %s", parent)
	}
	backup := filepath.Join(tempDir, "crash")
	if err := os.Rename(crashDir, backup); err != nil {
		return nil, errors.Wrapf(err, "failed to rename crash directory from %s to %s", crashDir, backup)
	}
	return func() error {
		// remove all existing files in crash directory and restore stashed ones.
		if err := os.RemoveAll(crashDir); err != nil {
			return errors.Wrapf(err, "failed to remove content of crash directory %s before restoring", crashDir)
		}
		if err := os.Rename(backup, crashDir); err != nil {
			return errors.Wrapf(err, "failed to restore crash directory from %s to %s", backup, crashDir)
		}
		if err := os.RemoveAll(tempDir); err != nil {
			return errors.Wrapf(err, "failed to remove temporary directory %s", tempDir)
		}
		return nil
	}, nil
}

// replaceCrashFilterIn replaces --filter_in= flag value of the crash reporter.
// When param is an empty string, the flag will be removed.
// The kernel is set up to call the crash reporter with the core dump as stdin
// when a process dies. This function adds a filter to the command line used to
// call the crash reporter. This is used to ignore crashes in which we have no
// interest.
func replaceCrashFilterIn(param string) error {
	b, err := ioutil.ReadFile(CorePattern)
	if err != nil {
		return errors.Wrapf(err, "failed reading core pattern file %s", CorePattern)
	}
	pattern := string(b)
	if !strings.HasPrefix(pattern, "|") {
		return errors.Wrapf(err, "pattern should start with '|', but was: %s", pattern)
	}
	e := strings.Split(strings.TrimSpace(pattern), " ")
	var newargs []string
	replaced := false
	for _, s := range e {
		if !strings.HasPrefix(s, "--filter_in=") {
			newargs = append(newargs, s)
			continue
		}
		if len(param) == 0 {
			// Remove from list.
			continue
		}
		newargs = append(newargs, "--filter_in="+param)
		replaced = true
	}
	if len(param) != 0 && !replaced {
		newargs = append(newargs, "--filter_in="+param)
	}
	pattern = strings.Join(newargs, " ")

	if err := ioutil.WriteFile(CorePattern, []byte(pattern), 0644); err != nil {
		return errors.Wrapf(err, "failed writing core pattern file %s", CorePattern)
	}
	return nil
}

// DisableCrashFiltering removes the --filter_in argument from the kernel core dump cmdline.
// Next time the crash reporter is invoked (due to a crash) it will not receive a
// --filter_in paramter.
func DisableCrashFiltering() error {
	return replaceCrashFilterIn("")
}

// resetRateLimiting resets the count of crash reports sent today.
// This clears the contents of the rate limiting directory which has
// the effect of reseting our count of crash reports sent.
func resetRateLimiting() error {
	if err := os.RemoveAll(crashSenderRateDir); err != nil {
		return errors.Wrapf(err, "failed cleaning crash sender rate dir %s", crashSenderRateDir)
	}
	return nil
}

// setUpTestCrashReporter initializes the crash reporter for test mode.
func setUpTestCrashReporter(ctx context.Context) error {
	// Remove the test status flag to catch real error while initializing and setting up crash reporter.
	if err := unsetCrashTestInProgress(); err != nil {
		return errors.Wrap(err, "failed before initializing crash reporter")
	}
	if err := testexec.CommandContext(ctx, CrashReporterPath, "--init").Run(); err != nil {
		return errors.Wrap(err, "failed to initialize crash reporter")
	}
	// Completely disable crash_reporter from generating crash dumps
	// while any tests are running, otherwise a crashy system can make
	// these tests flaky.
	if err := replaceCrashFilterIn("none"); err != nil {
		return errors.Wrap(err, "failed after initializing crash reporter")
	}
	// Set the test status flag to make crash reporter.
	if err := setCrashTestInProgress(); err != nil {
		return errors.Wrap(err, "failed after initializing crash reporter")
	}
	return nil
}

// teardownTestCrashReporter handles resetting some test-specific persistent changes to the system made by setUpTestCrashReporter.
func teardownTestCrashReporter() error {
	if err := DisableCrashFiltering(); err != nil {
		return errors.Wrap(err, "failed while tearing down crash reporter")
	}
	if err := unsetCrashTestInProgress(); err != nil {
		return errors.Wrap(err, "failed while tearing down crash reporter")
	}
	return nil
}

func waitForProcessEnd(ctx context.Context, name string) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		cmd := testexec.CommandContext(ctx, "pgrep", "-f", name)
		err := cmd.Run()
		if err == nil {
			return errors.New("still have a process")
		}
		code, err := exitCode(err)
		if err != nil {
			// Failed to extrat exit code.
			cmd.DumpLog(ctx)
			return testing.PollBreak(errors.Wrapf(err, "failed to get exit code of %s", name))
		}
		if code == 0 {
			// This will never happen. If return code is 0, cmd.Run indicates it by err==nil.
			return testing.PollBreak(errors.New("inconsistent results returned from cmd.Run()"))
		}
		return nil
	}, &testing.PollOptions{Timeout: time.Duration(10) * time.Second})
}

// runCrasherProcess runs the crasher process.
// Will wait up to 10 seconds for crash_reporter to finish.
func runCrasherProcess(ctx context.Context, opts CrasherOptions) (*CrasherResult, error) {
	var command []string
	if opts.Username != "root" {
		command = []string{"su", opts.Username, "-c"}
	}
	basename := filepath.Base(crasherPath)
	if err := replaceCrashFilterIn(basename); err != nil {
		return nil, errors.Wrapf(err, "failed to replace crash filter: %v", err)
	}
	command = append(command, crasherPath)
	if !opts.CauseCrash {
		command = append(command, "--nocrash")
	}
	oldConsent, err := metrics.HasConsent()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get existing consent status: %v", err)
	}
	if oldConsent != opts.Consent {
		metrics.SetConsent(ctx, TestCert, opts.Consent)
		defer metrics.SetConsent(ctx, TestCert, oldConsent)
	}
	cmd := testexec.CommandContext(ctx, command[0], command[1:]...)

	watcher, err := syslog.NewWatcher("/var/log/messages")
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare syslog watcher in runCrasherProcess")
	}

	crasherExitCode := 0
	b, err := cmd.CombinedOutput()
	out := string(b)
	if err != nil {
		var err2 error
		if crasherExitCode, err2 = exitCode(err); err2 != nil {
			return nil, errors.Wrapf(err2, "failed to get crasher exit code: %v", err)
		}
	}

	// Get the PID from the output, since |crasher.pid| may be su's PID.
	m := pidRegex.FindStringSubmatch(out)
	if m == nil {
		return nil, errors.Errorf("no PID found in output: %s", out)
	}
	pid, err := strconv.Atoi(m[1])
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse PID from output of command")
	}
	usr, err := user.Lookup(opts.Username)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to lookup username %s", opts.Username)
	}
	var reason string
	if opts.Consent {
		reason = "handling"
	} else {
		reason = "ignoring - no consent"
	}
	crashCaughtMessage := fmt.Sprintf(crashReporterLogFormat, basename, pid, usr.Uid, usr.Gid, reason)

	// Wait until no crash_reporter is running.
	if err := waitForProcessEnd(ctx, "crash_reporter.*:"+basename); err != nil {
		// TODO(crbug.com/970930): include system log message in this error.
		return nil, errors.Wrap(err, "timeout waiting for crash_reporter to finish: ")
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		cmd := testexec.CommandContext(ctx, "pgrep", "-f", "crash_reporter.*:"+basename)
		err := cmd.Run()
		if err == nil {
			return errors.New("still have a process")
		}
		code, err := exitCode(err)
		if err != nil {
			// Failed to extrat exit code.
			cmd.DumpLog(ctx)
			return testing.PollBreak(errors.Wrap(err, "failed to get exit code of crasher"))
		}
		if code == 0 {
			// This will never happen. If return code is 0, cmd.Run indicates it by err==nil.
			return testing.PollBreak(errors.New("inconsistent results returned from cmd.Run()"))
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		// TODO(yamaguchi): include log reader message in this error.
		return nil, errors.Wrap(err, "timeout waiting for crash_reporter to finish: ")
	}

	// Wait until crash reporter processes the crash, or making sure it didn't.
	c, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	err = watcher.WaitForMessage(c, crashCaughtMessage)
	var crashReporterCaught bool
	select {
	case <-c.Done():
		// Context timed out. This usually means WaitForMessage timed out.
		// However, there are two exceptional cases by race conditions.
		// 1. WaitForMessage returned non-timeout error (like I/O error) right before the context times out.
		// Ideally the test should catch it and fail.
		// However we don't distinguish such case with this normal path, because it can only happen after
		// multiple consequent successful reads verifying that the pattern did not appear in the log.
		// 2. WaitForMessage successfully found target message right before context timed out. Covered here.
		crashReporterCaught = err == nil
	default:
		if err != nil {
			return nil, errors.Wrap(err, "failed to verify crash_reporter message")
		}
		crashReporterCaught = true
	}

	var expectedExitCode int
	if opts.Username == "root" {
		// POSIX-style exit code for a signal.
		expectedExitCode = -int(syscall.SIGSEGV)
	} else {
		// Bash-style exit code for a signal (because it's run with "su -c").
		expectedExitCode = 128 + int(syscall.SIGSEGV)
	}
	result := CrasherResult{
		Crashed:             (crasherExitCode == expectedExitCode),
		CrashReporterCaught: crashReporterCaught,
		ReturnCode:          crasherExitCode,
	}
	testing.ContextLog(ctx, "Crasher process result: ", result)
	return &result, nil
}

func addDotPrefix(s []string) []string {
	var r []string
	for _, e := range s {
		r = append(r, "."+e)
	}
	return r
}

// RunCrasherProcessAndAnalyze executes a crasher process and extracts result data from dumps and logs.
func RunCrasherProcessAndAnalyze(ctx context.Context, opts CrasherOptions) (*CrasherResult, error) {
	result, err := runCrasherProcess(ctx, opts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute and capture result of crasher")
	}
	if !result.Crashed || !result.CrashReporterCaught {
		return result, nil
	}
	crashDir, err := getCrashDir(opts.Username)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get crash directory for user [%s]", opts.Username)
	}
	if !opts.Consent {
		if _, err := os.Stat(crashDir); err == nil || !os.IsNotExist(err) {
			return nil, errors.Wrap(err, "crash directory should not exist")
		}
		return result, nil
	}

	if info, err := os.Stat(crashDir); err != nil || !info.IsDir() {
		return nil, errors.Wrap(err, "crash directory does not exist")
	}

	crashDir = canonicalizeCrashDir(crashDir)
	crashContents, err := ioutil.ReadDir(crashDir)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read crash directory %s", crasherPath)
	}

	// The prefix of report file names. Basename of the executable, but non-alphanumerics replaced by underscores.
	// See CrashCollector::Sanitize in src/platform2/crash-repoter/crash_collector.cc.
	basename := nonAlphaNumericRegex.ReplaceAllLiteralString(filepath.Base(crasherPath), "_")

	// A dict tracking files for each crash report.
	crashReportFiles := make(map[string]string)
	if err := checkCrashDirectoryPermissions(crashDir); err != nil {
		return result, err
	}

	testing.ContextLogf(ctx, "Contents in %s: %s", crashDir, crashContents)

	// Variables and their typical contents:
	// basename: crasher_nobreakpad
	// filename: crasher_nobreakpad.20181023.135339.16890.dmp
	// ext: .dmp
	for _, f := range crashContents {
		if filepath.Ext(f.Name()) == ".core" {
			// Ignore core files.  We'll test them later.
			continue
		}
		if !strings.HasPrefix(f.Name(), basename+".") {
			// Flag all unknown files.
			return nil, errors.Errorf("crash reporter created an unknown file: %s / base=%s",
				f.Name(), basename)
		}
		ext := filepath.Ext(f.Name())
		testing.ContextLogf(ctx, "Found crash report file (%s): %s", ext, f.Name())
		if data, ok := crashReportFiles[ext]; ok {
			return nil, errors.Errorf("found multiple files with %s: %s and %s",
				ext, f.Name(), data)
		}
		crashReportFiles[ext] = f.Name()
	}

	// Every crash report needs one of these to be valid.
	reportRequiredFiletypes := []string{
		".meta",
	}
	// Reports might have these and that's OK!
	reportOptionalFiletypes := []string{
		".dmp", ".log", ".proclog",
	}

	// Make sure we generated the exact set of files we expected.
	var missingFileTypes []string
	for _, rt := range reportRequiredFiletypes {
		found := false
		for k := range crashReportFiles {
			if k == rt {
				found = true
				break
			}
		}
		if !found {
			missingFileTypes = append(missingFileTypes, rt)
		}
	}
	if len(missingFileTypes) > 0 {
		return nil, errors.Errorf("crash report is missing files: %v", missingFileTypes)
	}

	find := func(target string, lst []string) bool {
		for _, v := range lst {
			if target == v {
				return true
			}
		}
		return false
	}
	findInKey := func(target string, m map[string]string) bool {
		_, found := m[target]
		return found
	}

	var unknownFileTypes []string
	for k := range crashReportFiles {
		if !find(k, append(reportRequiredFiletypes, reportOptionalFiletypes...)) {
			unknownFileTypes = append(missingFileTypes, k)
		}
	}
	if len(unknownFileTypes) > 0 {
		return nil, errors.Errorf("crash report includes unkown files: %v", unknownFileTypes)
	}

	// Create full paths for the logging code below.
	for _, key := range append(reportRequiredFiletypes, reportOptionalFiletypes...) {
		if findInKey(key, crashReportFiles) {
			crashReportFiles[key] = filepath.Join(crashDir, crashReportFiles[key])
		} else {
			crashReportFiles[key] = ""
		}
	}

	result.Minidump = crashReportFiles[".dmp"]
	result.Basename = filepath.Base(crasherPath)
	result.Meta = crashReportFiles[".meta"]
	result.Log = crashReportFiles[".log"]
	return result, nil
}

// isFrameInStack searches for frame entries in the given stack dump text.
// Returns true if an exact match is present.
//
// A frame entry looks like (alone on a line)
// "16  crasher_nobreakpad!main [crasher.cc : 21 + 0xb]",
// where 16 is the frame index (0 is innermost frame),
// crasher_nobreakpad is the module name (executable or dso), main is the function name,
// crasher.cc is the function name and 21 is the line number.
//
// We do not care about the full function signature - ie, is it
// foo or foo(ClassA *).  These are present in function names
// pulled by dump_syms for Stabs but not for DWARF.
func isFrameInStack(ctx context.Context, frameIndex int, moduleName, functionName, fileName string,
	lineNumber int, stack []byte) bool {
	re := regexp.MustCompile(
		fmt.Sprintf(`\n\s*%d\s+%s!%s.*\[\s*%s\s*:\s*%d\s.*\]`,
			frameIndex, moduleName, functionName, fileName, lineNumber))
	testing.ContextLog(ctx, "Searching for regexp ", re)
	return re.FindSubmatch(stack) != nil
}

// verifyStack checks if a crash happened at the expected location.
func verifyStack(ctx context.Context, stack []byte, basename string, fromCrashReporter bool) error {
	testing.ContextLogf(ctx, "minidump_stackwalk output: %s", string(stack))

	// Look for a line like:
	// Crash reason:  SIGSEGV
	// Crash reason:  SIGSEGV /0x00000000
	match := regexp.MustCompile(`Crash reason:\s+([^\s]*)`).FindSubmatch(stack)
	const expectedAddress = "0x16"
	if match == nil || string(match[1]) != "SIGSEGV" {
		return errors.New("Did not identify SIGSEGV cause")
	}

	match = regexp.MustCompile(`Crash address:\s+(.*)`).FindSubmatch(stack)
	if match == nil || string(match[1]) != expectedAddress {
		return errors.Errorf("Did not identify crash address %s", expectedAddress)
	}

	const (
		bombSource    = `platform\.UserCrash\.crasher\.bomb\.cc`
		crasherSource = `platform\.UserCrash\.crasher\.crasher\.cc`
		recbomb       = "recbomb"
	)

	// Should identify crash at *(char*)0x16 assignment line.
	if !isFrameInStack(ctx, 0, basename, recbomb, bombSource, 9, stack) {
		return errors.New("Did not show crash line on stack")
	}

	// Should identify recursion line which is on the stack for 15 levels.
	if !isFrameInStack(ctx, 15, basename, recbomb, bombSource, 12, stack) {
		return errors.New("Did not show recursion line on stack")
	}

	// Should identify main line.
	if !isFrameInStack(ctx, 16, basename, "main", crasherSource, 23, stack) {
		return errors.New("Did not show main on stack")
	}
	return nil
}

// setSendingMock enables / disables mocking of the sending process.
// This uses the _MOCK_CRASH_SENDING file to achieve its aims. See notes
// at the top.
// @param mock_enabled: If True, mocking is enabled, else it is disabled.
// @param send_success: If mock_enabled this is True for the mocking to
// 		indicate success, False to indicate failure.
func setSendingMock(enableMock bool, sendSuccess bool) error {
	if enableMock {
		var data string
		if sendSuccess {
			data = ""
		} else {
			data = "1"
		}
		// 	logging.info('Setting sending mock')
		if err := ioutil.WriteFile(mockCrashSending, []byte(data), 0644); err != nil {
			return errors.Wrap(err, "failed to create pause file")
		}
	} else {
		if err := os.Remove(mockCrashSending); err != nil && !os.IsNotExist(err) {
			return errors.Wrapf(err, "failed to remove mock crash file %s", mockCrashSending)
		}
	}
	return nil
}

// getDmpContents creates the contents of the dmp file for our made crashes.
// The dmp file contents are deliberately large and hard-to-compress. This
// ensures logging_CrashSender hits its bytes/day cap before its sends/day
// cap.
func getDmpContents() []byte {
	// Matches kDefaultMaxUploadBytes
	const maxCrashSize = 1024 * 1024
	result := make([]byte, maxCrashSize, maxCrashSize)
	rand.Read(result)
	return result
}

// writeCrashDirEntry Writes a file to the system crash directory.
// This writes a file to _SYSTEM_CRASH_DIR with the given name. This is
// used to insert new crash dump files for testing purposes.
// @param name: Name of file to write.
// @param contents: String to write to the file.
func writeCrashDirEntry(name string, contents []byte) (string, error) {
	entry, err := getCrashDir(name)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get crash dir for user %s", name)
	}
	_, err = os.Stat(systemCrashDir)
	if err != nil && os.IsNotExist(err) {
		if err := os.Mkdir(systemCrashDir, os.FileMode(0770)); err != nil {
			return "", errors.Wrapf(err, "failed to create crash directory %s", systemCrashDir)
		}
	}
	if err := ioutil.WriteFile(entry, contents, 0660); err != nil {
		return "", errors.Wrap(err, "failed to write crash dir entry")
	}
	return entry, nil
}

// writeFakeMeta writes a fake meta entry to the system crash directory.
// @param name: Name of file to write.
// @param exec_name: Value for exec_name item.
// @param payload: Value for payload item.
// @param complete: True to close off the record, otherwise leave it
// 		incomplete.
func writeFakeMeta(name string, execName string, payload string) (string, error) {
	contents := fmt.Sprintf("exec_name=%s\n"+
		"ver=my_ver\n"+
		"payload=%s\n"+
		"done=1\n",
		execName, payload)
	return writeCrashDirEntry(name, []byte(contents))
}

// prepareSenderOneCrash creates metadata for a fake crash report.
// This enabled mocking of the crash sender, then creates a fake
// crash report for testing purposes.
//
// @param send_success: True to make the crash_sender success, False to
// make it fail.
// @param reports_enabled: True to enable consent to that reports will be
// sent.
// @param report: Report to use for crash, if None we create one.
func prepareSenderOneCrash(ctx context.Context, sendSuccess bool, reportsEnabled bool, report string) (string, error) {
	// Use the same file format as crash does normally:
	// <basename>.#.#.#.meta
	const fakeTestBasename = "fake.1.2.3"
	setSendingMock(true /* mock_enabled */, sendSuccess)
	metrics.SetConsent(ctx, TestCert, reportsEnabled)
	if report == "" {
		// Use the same file format as crash does normally:
		// <basename>.#.#.#.meta
		payload, err := writeCrashDirEntry(fmt.Sprintf("%s.dmp", fakeTestBasename), getDmpContents())
		if err != nil {
			return "", errors.Wrap(err, "fail while preparing sender one crash")
		}
		report, err = writeFakeMeta(fmt.Sprintf("%s.meta", fakeTestBasename), "fake", payload)
		if err != nil {
			// TODO: better message
			return "", errors.Wrap(err, "fail while preparing sender one crash")
		}
	}
	return report, nil
}

// SenderOutput represents data extracted from crash sender execution result.
type SenderOutput struct {
	ExecName      string // name of executable which crashed
	ImageType     string // type of image ("dev","test",...), if given
	BootMode      string // current boot mode ("dev",...), if given
	MetaPath      string // path to the report metadata file
	Output        string // the output from the script, copied
	ReportKind    string // kind of report sent (minidump vs kernel)
	ReportPayload string // payload of report sent
	SendAttempt   bool   // did the script attempt to send a crash.
	SendSuccess   bool   // if it attempted, was the crash send successful.
	Sig           string // signature of the report, if given.
	SleepTime     int    // if it attempted, how long did it sleep before
	Sending       int    // (if mocked, how long would it have slept)

	// ReportExists is whether the minidump still exist after calling send script.
	ReportExists bool

	// RateCount is number of crashes that have been uploaded in the past 24 hours.
	RateCount int
}

// parseSenderOutput parses the log output from the crash_sender script.
// This script can run on the logs from either a mocked or true
// crash send. It looks for one and only one crash from output.
// Non-crash anomalies should be ignored since there're just noise
// during running the test.
func parseSenderOutput(output string) (*SenderOutput, error) {
	anomalyTypes := []string{
		"kernel_suspend_warning",
		"kernel_warning",
		"kernel_wifi_warning",
		"selinux_violation",
		"service_failure",
	}
	// 	"""Narrow search to lines from crash_sender."""
	var crashSenderSearch = func(pattern string, output string) []int {
		// TODO: handle error
		return regexp.MustCompile(`crash_sender\[\d+\]:\s+` + pattern).FindStringSubmatchIndex(output)
	}
	// https://cs.corp.google.com/chromeos_public/src/third_party/autotest/files/client/cros/crash/crash_test.py?q=call_sender_one_crash&g=0&l=388
	beforeFirstCrash := "" // None
	isAnormaly := func(s string) bool {
		for _, a := range anomalyTypes {
			if strings.Contains(s, a) {
				return true
			}
		}
		return false
	}

	for {
		crashHeader := crashSenderSearch(`Considering metadata (\S+)`, output)
		if crashHeader == nil {
			break
		}
		if beforeFirstCrash == "" {
			beforeFirstCrash = output[0:crashHeader[0]]
		}
		// TODO: check array size
		metaConsidered := output[crashHeader[0]:crashHeader[1]]
		if isAnormaly(metaConsidered) {
			// If it's an anomaly, skip this header, and look for next one.
			output = output[crashHeader[1]:]
		} else {
			// If it's not an anomaly, skip everything before this header.
			output = output[crashHeader[0]:]
			break
		}
	}

	if beforeFirstCrash != "" {
		output = beforeFirstCrash + output
		// logging.debug('Filtered sender output to parse:\n%s', output)
	}

	sleepMatch := crashSenderSearch(`Scheduled to send in (\d+)s`, output)
	sendAttempt := sleepMatch != nil
	var sleepTime int
	if sendAttempt {
		var err error
		s := output[sleepMatch[0]:sleepMatch[1]]
		sleepTime, err = strconv.Atoi(s)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid sleep time in log: %s", s)
		}
	} else {
		sleepTime = -1 // None
	}

	// meta_match = crash_sender_search('Metadata: (\S+) \((\S+)\)', output)
	// if meta_match:
	// 	meta_path = meta_match.group(1)
	// 	report_kind = meta_match.group(2)
	// else:
	// 	meta_path = None
	// 	report_kind = None

	// payload_match = crash_sender_search('Payload: (\S+)', output)
	// if payload_match:
	// 	report_payload = payload_match.group(1)
	// else:
	// 	report_payload = None

	// exec_name_match = crash_sender_search('Exec name: (\S+)', output)
	// if exec_name_match:
	// 	exec_name = exec_name_match.group(1)
	// else:
	// 	exec_name = None

	// sig_match = crash_sender_search('sig: (\S+)', output)
	// if sig_match:
	// 	sig = sig_match.group(1)
	// else:
	// 	sig = None

	// image_type_match = crash_sender_search('Image type: (\S+)', output)
	// if image_type_match:
	// 	image_type = image_type_match.group(1)
	// else:
	// 	image_type = None

	// boot_mode_match = crash_sender_search('Boot mode: (\S+)', output)
	// if boot_mode_match:
	// 	boot_mode = boot_mode_match.group(1)
	// else:
	// 	boot_mode = None

	// send_success = 'Mocking successful send' in output
	return &SenderOutput{
		// ExecName:   execName,
		// ReportKind: reportKind,
		// MetaPath: metaPath,

		// 'report_payload': report_payload,
		// 'send_attempt': send_attempt,
		// 'send_success': send_success,
		// 'sig': sig,
		// 'image_type': image_type,
		// 'boot_mode': boot_mode,
		SleepTime: sleepTime,
		Output:    output,
	}, nil
}

// senderOptions contains options for callSenderOneCrash.
type senderOptions struct {
	SendSuccess    bool   // Mock a successful send if true
	ReportsEnabled bool   // Has the user consented to sending crash reports.
	Report         string // report to use for crash, if --None-- we create one.
	ShouldFail     bool   // expect the crash_sender program to fail
	IgnorePause    bool   // crash_sender should ignore pause file existence
}

// DefaultSenderOptions creates a senderOptions object with default values.
func DefaultSenderOptions() senderOptions {
	return senderOptions{
		SendSuccess:    true,
		ReportsEnabled: true,
		ShouldFail:     false,
		IgnorePause:    true,
	}
}

func waitForSenderCompletion(ctx context.Context, watcher *syslog.Watcher) error {
	// Wait for no crash_sender's last message to be placed in the
	// system log before continuing and for the process to finish.
	// Otherwise we might get only part of the output.
	// 	timeout=60,
	// TODO: set ctx timeout
	err := watcher.WaitForMessage(ctx, "crash_sender done.")
	if err != nil {
		return errors.Wrapf(err, "Timeout waiting for crash_sender to emit done: %s",
			"") //// 	  self._log_reader.get_logs()))
		// TODO: add log content
	}
	if err := waitForProcessEnd(ctx, "crash_sender"); err != nil {
		// 	TODO: set timeout. timeout=60
		return errors.Wrap(err, "Timeout waiting for crash_sender to finish: ")
		// TODO: add log content
		// 		+ self._log_reader.get_logs()))
	}
	return nil
}

// checkMinidumpStackwalk acquires stack dump log from minidump and verifies it.
func checkMinidumpStackwalk(ctx context.Context, minidumpPath, basename string, fromCrashReporter bool) error {
	symbolDir := filepath.Join(filepath.Dir(crasherPath), "symbols")
	command := []string{"minidump_stackwalk", minidumpPath, symbolDir}
	cmd := testexec.CommandContext(ctx, command[0], command[1:]...)
	out, err := cmd.CombinedOutput(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrapf(err, "failed to get minidump output %v", cmd)
	}
	if err := verifyStack(ctx, out, basename, fromCrashReporter); err != nil {
		return errors.Wrap(err, "minidump stackwalk verification failed")
	}
	return nil
}

// callSenderOneCrash calls the crash sender script to mock upload one crash.
func callSenderOneCrash(ctx context.Context, opts senderOptions) (*SenderOutput, error) {
	report, err := prepareSenderOneCrash(ctx, opts.SendSuccess, opts.ReportsEnabled, opts.Report)
	if err != nil {
		return nil, errors.Wrap(err, "failed toprepare senderOneCrash")
	}
	w, err := syslog.NewWatcher(messagesFile)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create log watcher for %s", messagesFile)
	}

	var option string
	if opts.IgnorePause {
		option = "--ignore_pause_file"
	}
	cmd := testexec.CommandContext(ctx, crashSenderPath, option)
	scriptOutput, err := cmd.CombinedOutput()
	code, err := exitCode(err)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get exit code for cmd=%v", cmd)
	}
	if code != 0 && !opts.ShouldFail {
		return nil, errors.Errorf("%q returned an unexpected non-zero value (%s)", cmd, code)
	}

	if err := waitForSenderCompletion(ctx, w); err != nil {
		return nil, err
	}
	output, err := w.GetLogs()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get logs")
	}

	if string(scriptOutput) != "" {
		testing.ContextLogf(ctx, "crash_sender stdout/stderr: %s", scriptOutput)
	}

	var reportExists bool
	fileInfo, err := os.Stat(report)
	if err != nil && !os.IsNotExist(err) {
		return nil, errors.Wrap(err, "failed to stat report file")
	}
	if err == nil {
		if fileInfo.IsDir() {
			// TODO: should we remove the dir and continue as reportExists=false?
			return nil, errors.Errorf("%s is a directory", report)
		}
		reportExists = true
		os.Remove(report)
	}

	var rateCount int
	fileInfo, err = os.Stat(crashSenderRateDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, errors.Wrap(err, "failed to stat crash sender rate directory")
	}
	if err == nil {
		files, err := ioutil.ReadDir(crashSenderRateDir)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read crash sender rate directory")
		}
		for _, f := range files {
			if f.Mode().IsRegular() {
				rateCount++
			}
		}
	}

	result, err := parseSenderOutput(*output)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse sender output")
	}

	result.ReportExists = reportExists
	result.RateCount = rateCount

	// Show the result for debugging but remove 'output' field
	// since it's large and earlier in debug output.
	debugResult := result
	debugResult.Output = ""
	testing.ContextLog(ctx, "Result of send (besides output): ", debugResult)

	return result, nil

}

func checkGeneratedReportSending(ctx context.Context, metaPath, payloadPath, execName, reportKind, expectedSig string) error {
	// Now check that the sending works
	o := DefaultSenderOptions()
	o.Report = filepath.Base(payloadPath)
	result, err := callSenderOneCrash(ctx, o)
	if err != nil {
		return errors.Wrap(err, "failed to call sender one crash")
	}
	if !result.SendAttempt || !result.SendSuccess || result.ReportExists {
		// TODO: explain more
		return errors.New("Report not sent properly")
	}
	if result.ExecName != execName {
		return errors.Errorf("executable name incorrect: want %s, got %s", execName, result.ExecName)
	}
	if result.ReportKind != reportKind {
		return errors.Errorf("Wrong report type: want %s, got %s", reportKind, result.ReportKind)
	}
	if result.ReportPayload != payloadPath {
		return errors.Errorf("Sent the wrong minidump payload: want %s, got %s", payloadPath, result.ReportPayload)
	}
	if result.MetaPath != metaPath {
		return errors.Errorf("Used the wrong meta file: want %s, got %s", metaPath, result.MetaPath)
	}
	if expectedSig == "" {
		if result.Sig != "" {
			return errors.New("Report should not have signature")
		}
	} else if result.Sig != expectedSig {
		// TODO: check if Sig can be absent in Python
		return errors.Errorf("Report signature mismatch: want %s, got %s", expectedSig, result.Sig)
	}
	// version :=

	// version = self._expected_version
	// if version is None:
	// 	lsb_release = utils.read_file('/etc/lsb-release')
	// 	version = re.search(
	// 		r'CHROMEOS_RELEASE_VERSION=(.*)', lsb_release).group(1)

	// if not ('Version: %s' % version) in result['output']:
	// 	raise error.TestFail('Missing version %s in log output' % version)}
	return nil
}

// CheckCrashingProcess runs crasher process and verifies that it's processed.
func CheckCrashingProcess(ctx context.Context, opts CrasherOptions) error {
	restoreCrashFiles, err := stashCrashFiles(opts.Username)
	if err != nil {
		return errors.Wrap(err, "failed to stash crash files")
	}
	defer restoreCrashFiles()
	result, err := RunCrasherProcessAndAnalyze(ctx, opts)
	if err != nil {
		return errors.Wrap(err, "failed to run and analyze crasher")
	}
	if !result.Crashed {
		return errors.Errorf("Crasher returned %d instead of crashing", result.ReturnCode)
	}
	if !result.CrashReporterCaught {
		return errors.New("Logs do not contain crash_reporter message")
	}
	if !opts.Consent {
		return nil
	}
	if result.Minidump == "" {
		return errors.New("crash reporter did not generate minidump")
	}

	// TODO(crbug.com/970930): Check that crash reporter announces minidump location to the log like "Stored minidump to /var/...."

	if err := checkMinidumpStackwalk(ctx, result.Minidump, result.Basename, true); err != nil {
		return err
	}

	// TODO(crbug.com/970930): Check that generated report is sent.

	checkGeneratedReportSending(ctx, result.Meta, result.Minidump, result.Basename, "minidump", "")

	return nil
}

func runCrashTest(ctx context.Context, s *testing.State, testFunc func(context.Context, *testing.State), initialize bool) error {
	if initialize {
		if err := setUpTestCrashReporter(ctx); err != nil {
			return err
		}
		defer teardownTestCrashReporter()
	}
	// Disable crash_sender from running, kill off any running ones.
	// We set a flag to crash_sender when invoking it manually to avoid
	// our invocations being paused.
	if err := disableSystemSending(); err != nil {
		return err
	}
	defer enableSystemSending()
	// Ignore process-not-found error.
	// TODO(yamaguchi): Refactor to this after Go version >= 1.12
	// (*cmd.ProcessState).ExitCode()
	if err := testexec.CommandContext(ctx, "pkill", "-9", "-e", "crash_sender").Run(); err != nil {
		e, ok := err.(*exec.ExitError)
		if !ok {
			return errors.Wrap(err, "failed to get exit status from crash_sender: failed to cast to exec.ExitError")
		}
		s, ok := e.Sys().(syscall.WaitStatus)
		if !ok {
			return errors.Wrap(err, "failed to get exit status from crash_sender: failed to cast to syscall.WaitStatus")
		}
		if s.ExitStatus() != 1 {
			return errors.Wrap(err, "failed to kill crash_sender")
		}
	}
	resetRateLimiting()
	testFunc(ctx, s)
	return nil
}

// RunCrashTests runs crash test cases after setting up crash reporter.
func RunCrashTests(ctx context.Context, s *testing.State, testFuncList []func(context.Context, *testing.State), initialize bool) {
	for _, f := range testFuncList {
		runCrashTest(ctx, s, f, initialize)
	}
}
