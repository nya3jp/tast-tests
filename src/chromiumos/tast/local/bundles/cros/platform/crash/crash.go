// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package crash contains functionality shared by tests that exercise the crash reporter.
package crash

import (
	"context"
	"io/ioutil"
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
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/metrics"
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

	crasherPath        = "/usr/libexec/tast/helpers/local/cros/platform.UserCrash.crasher"
	crashSenderRateDir = "/var/lib/crash_sender"
	pauseFile          = "/var/lib/crash_sender_paused"

	systemCrashDir       = "/var/spool/crash"
	fallbackUserCrashDir = "/home/chronos/crash"
	userCrashDirs        = "/home/chronos/u-*/crash"
	userCrashDirRegex    = "/home/chronos/u-([a-f0-9]+)/crash"
	backupCrashDir       = "/tmp/crashTestBackup"
)

// CrasherOptions stores configurations for running crasher process.
type CrasherOptions struct {
	Username   string
	CauseCrash bool
	Consent    bool
}

// CrasherResult stores result status and outputs from a crasher prcess execution.
type CrasherResult struct {
	ReturnCode          int
	Crashed             bool
	CrashReporterCaught bool
	Output              string
	Minidump            string
	Basename            string
	Meta                string
	Log                 string
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
func exitCode(err error) (int, error) {
	e, ok := err.(*exec.ExitError)
	if !ok {
		return 0, errors.Wrap(err, "failed to cast to exec.ExitError")
	}
	s, ok := e.Sys().(syscall.WaitStatus)
	if !ok {
		return 0, errors.Wrap(err, "failed to cast to syscall.WaitStatus")
	}
	if s.Exited() {
		return s.ExitStatus(), nil
	}
	if !s.Signaled() {
		return 0, errors.Wrap(err, "unexpected exit status")
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
	// mode = stat.S_IMODE(stat_info.st_mode)
	permittedModes := make(map[os.FileMode]struct{})
	var expectedUser string
	var expectedGroup string
	mode := fileInfo.Mode()
	if strings.HasPrefix(path, "/var/spool/crash") {
		if fileInfo.IsDir() {
			// utils.system('ls -l %s' % crash_dir) // original outputs this as a log, maybe for debugging
			files, err := ioutil.ReadDir(path)
			if err != nil {
				return errors.Wrapf(err, "failed to read directory %s", path)
			}
			for _, f := range files {
				if err := checkCrashDirectoryPermissions(filepath.Join(path, f.Name())); err != nil {
					return err
				}
			}
			// TODO: check if we need to verify gid bit.
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
		return errors.Errorf("expected %s.%s ownership of %s (actual %s.%s)",
			expectedUser, expectedGroup, path, usr.Username, grp.Name)
	}
	if _, found := permittedModes[mode]; !found {
		extractKeys := func(m map[os.FileMode]struct{}) []os.FileMode {
			var result []os.FileMode
			for k := range m {
				result = append(result, k)
			}
			return result
		}
		return errors.Errorf("expected %s to have mode in %v (actual %v)",
			path, extractKeys(permittedModes), mode)
	}
	return nil
}

func getCrashDir(username string) string {
	if username == "root" || username == "crash" {
		return systemCrashDir
	}
	p, _ := filepath.Glob(userCrashDirs)
	// Omitting error handling because it happens only when userCrashDirs is malformed
	if len(p) == 0 {
		return fallbackUserCrashDir
	}
	return p[0]
}

// canonicalizeCrashDir converts /home/chronos crash directory to /home/user counterpart.
func canonicalizeCrashDir(path string) string {
	r := regexp.MustCompile(userCrashDirRegex)
	m := r.FindStringSubmatch(path)
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

func stashCrashFiles(userName string) error {
	path := getCrashDir(userName)
	os.RemoveAll(backupCrashDir)
	os.Mkdir(backupCrashDir, os.FileMode(0770))
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return errors.Wrapf(err, "failed to read crash directory %s", path)
	}
	for _, f := range files {
		if err := fsutil.MoveFile(filepath.Join(path, f.Name()), filepath.Join(backupCrashDir, f.Name())); err != nil {
			return errors.Wrapf(err, "failed to move a file in crash directory: %s", f.Name())
		}
	}
	return nil
}

// restoreCrashFiles removes all existing files in crash directory and restore stashed ones.
func restoreCrashFiles(ctx context.Context, userName string) error {
	crashDir := getCrashDir(userName)
	files, err := ioutil.ReadDir(crashDir)
	if err != nil {
		return errors.Wrapf(err, "failed to read crash directory %s", crashDir)
	}
	for _, f := range files {
		if err := os.Remove(filepath.Join(crashDir, f.Name())); err != nil {
			return errors.Wrapf(err, "failed to delete a file in crash directory: %s", f)
		}
	}

	bf, err := ioutil.ReadDir(backupCrashDir)
	if err != nil {
		return errors.Wrapf(err, "failed to read crash backup directory %s", backupCrashDir)
	}
	for _, f := range bf {
		if err := fsutil.MoveFile(filepath.Join(backupCrashDir, f.Name()), filepath.Join(crashDir, f.Name())); err != nil {
			return errors.Wrapf(err, "failed to delete a file in crash directory: %s", f.Name())
		}
	}
	return os.RemoveAll(backupCrashDir)
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
			// remove from list
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

// initializeCrashReporter starts up the crash reporter.
func initializeCrashReporter(ctx context.Context) error {
	if err := testexec.CommandContext(ctx, CrashReporterPath, "--init").Run(); err != nil {
		return errors.Wrap(err, "failed to initialize crash reporter")
	}
	// Completely disable crash_reporter from generating crash dumps
	// while any tests are running, otherwise a crashy system can make
	// these tests flaky.
	return replaceCrashFilterIn("none")
}

// runCrasherProcess runs the crasher process.
// Will wait up to 10 seconds for crash_reporter to finish.
func runCrasherProcess(ctx context.Context, opts CrasherOptions) (CrasherResult, error) {
	var command []string
	if opts.Username != "root" {
		command = []string{"su", opts.Username, "-c"}
	}
	basename := filepath.Base(crasherPath)
	if err := replaceCrashFilterIn(basename); err != nil {
		return CrasherResult{}, errors.Wrapf(err, "failed to replace crash filter: %v", err)
	}
	command = append(command, crasherPath)
	if !opts.CauseCrash {
		command = append(command, "--nocrash")
	}
	oldConsent, err := metrics.HasConsent()
	if err != nil {
		return CrasherResult{}, errors.Wrapf(err, "failed to get existing consent status: %v", err)
	}
	if oldConsent != opts.Consent {
		metrics.SetConsent(ctx, TestCert, opts.Consent)
		defer metrics.SetConsent(ctx, TestCert, oldConsent)
	}
	cmd := testexec.CommandContext(ctx, command[0], command[1:]...)

	out, err := cmd.CombinedOutput()
	var crasherExitCode int
	if err != nil {
		var err2 error
		crasherExitCode, err2 = exitCode(err)
		if err2 != nil {
			return CrasherResult{}, errors.Wrapf(err2, "failed to get crasher exit code: %v", err)
		}
	} else {
		crasherExitCode = 0
	}

	// Wait until no crash_reporter is running.
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
	}, &testing.PollOptions{Timeout: time.Duration(10) * time.Second}); err != nil {
		// TODO(yamaguchi): include log reader message in this error.
		return CrasherResult{}, errors.Wrap(err, "timeout waiting for crash_reporter to finish: ")
	}

	var expectedExitCode int
	if opts.Username == "root" {
		// POSIX-style exit code for a signal
		expectedExitCode = -int(syscall.SIGSEGV)
	} else {
		// bash-style exit code for a signal (because it's run with "su -c")
		expectedExitCode = 128 + int(syscall.SIGSEGV)
	}
	result := CrasherResult{
		Crashed:    (crasherExitCode == expectedExitCode),
		Output:     string(out),
		ReturnCode: crasherExitCode,
	}
	testing.ContextLog(ctx, "Crasher process result: ", result)
	return result, nil
}

// RunCrasherProcessAndAnalyze executes a crasher process and extracts result data from dumps and logs.
func RunCrasherProcessAndAnalyze(ctx context.Context, opts CrasherOptions) (CrasherResult, error) {
	result, err := runCrasherProcess(ctx, opts)
	if err != nil {
		return result, errors.Wrap(err, "failed to execute and capture result of crasher: ")
	}
	// TODO(yamaguchi): implement syslog reader and verify crash based on it as well.
	if !result.Crashed /* || !result.CrashReporterCaught */ {
		return result, nil
	}
	if !opts.Consent {
		// We clear the crash diretory before test run, so that this test is not affecte by other tests.
		// Especially, there's a global limit with # of spooled files, which can make this test flaky depending on other tests..
		if _, err := os.Stat(getCrashDir(opts.Username)); err == nil || !os.IsNotExist(err) {
			return result, errors.Wrap(err, "Crash directory should not exist")
		}
		return result, nil
	}

	if info, err := os.Stat(getCrashDir(opts.Username)); err != nil || !info.IsDir() {
		return result, errors.Wrap(err, "Crash directory does not exist")
	}

	crashDir := canonicalizeCrashDir(getCrashDir(opts.Username))
	crashContents, err := ioutil.ReadDir(crashDir)
	if err != nil {
		return result, errors.Wrapf(err, "failed to read crash directory %s", crasherPath)
	}
	basename := strings.Replace(filepath.Base(crasherPath), ".", "_", -1)

	// A dict tracking files for each crash report.
	crashReportFiles := make(map[string]string)
	if err := checkCrashDirectoryPermissions(crashDir); err != nil {
		return result, err
	}

	testing.ContextLogf(ctx, "Contents in %s: %s", crashDir, crashContents)

	// Variables and their typical contents:
	// basename: crasher_nobreakpad
	// filename: crasher_nobreakpad.20181023.135339.16890.dmp
	// ext: dmp
	for _, f := range crashContents {
		if strings.HasSuffix(f.Name(), ".core") {
			// Ignore core files.  We'll test them later.
			continue
		}
		if strings.HasPrefix(f.Name(), basename+".") {
			c := strings.Split(f.Name(), ".")
			ext := c[len(c)-1]
			testing.ContextLogf(ctx, "Found crash report file (%s): %s", ext, f.Name())
			if data, ok := crashReportFiles[ext]; ok {
				return result, errors.Errorf("found multiple files with .%s: %s and %s",
					ext, f.Name(), data)
			}
			crashReportFiles[ext] = f.Name()
		} else {
			// Flag all unknown files.
			return result, errors.Errorf("Crash reporter created an unknown file: %s / base=%s",
				f.Name(), basename)
		}
	}

	// Every crash report needs one of these to be valid.
	reportRequiredFiletypes := []string{
		"meta",
	}
	// Reports might have these and that's OK!
	reportOptionalFiletypes := []string{
		"dmp", "log", "proclog",
	}

	// Make sure we generated the exact set of files we expected.
	foundFiletypes := crashReportFiles
	var missingFileTypes []string
	for _, rt := range reportRequiredFiletypes {
		found := false
		for k := range foundFiletypes {
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
		// TODO: message should add . before each element in missingFileTypes
		return result, errors.Errorf("crash report is missing files: %v", missingFileTypes)
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
	for k := range foundFiletypes {
		if !find(k, append(reportRequiredFiletypes, reportOptionalFiletypes...)) {
			unknownFileTypes = append(missingFileTypes, k)
		}
	}
	if len(unknownFileTypes) > 0 {
		// TODO: message should add . before each element in missingFileTypes
		return result, errors.Errorf("crash report includes unkown files: %v", unknownFileTypes)
	}

	// Create full paths for the logging code below.
	for _, key := range append(reportRequiredFiletypes, reportOptionalFiletypes...) {
		if findInKey(key, foundFiletypes) {
			crashReportFiles[key] = filepath.Join(crashDir, crashReportFiles[key])
		} else {
			crashReportFiles[key] = ""
		}
	}

	result.Minidump = crashReportFiles["dmp"]
	result.Basename = basename
	result.Meta = crashReportFiles["meta"]
	result.Log = crashReportFiles["log"]
	return result, nil
}

// CheckCrashingProcess runs crasher process and verifies that it's processed.
func CheckCrashingProcess(ctx context.Context, opts CrasherOptions) error {
	if err := stashCrashFiles(opts.Username); err != nil {
		return errors.Wrap(err, "failed to stash crash files")
	}
	defer restoreCrashFiles(ctx, opts.Username)
	result, err := RunCrasherProcessAndAnalyze(ctx, opts)
	if err != nil {
		return errors.Wrap(err, "failed to run and analyze crasher")
	}
	if !result.Crashed {
		return errors.Errorf("Crasher returned %d instead of crashing", result.ReturnCode)
	}

	// TODO(crbug.com/970930): Check if crash_reporter caught the crash.

	if !opts.Consent {
		return nil
	}
	if result.Minidump == "" {
		return errors.New("crash reporter did not announce minidump")
	}

	// TODO(crbug.com/970930): Check minidump stack walk.
	// TODO(crbug.com/970930): Check generated report sending.

	return nil
}

func runCrashTest(ctx context.Context, s *testing.State, testFunc func(context.Context, *testing.State), initialize bool) error {
	if initialize {
		if err := initializeCrashReporter(ctx); err != nil {
			return err
		}
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
