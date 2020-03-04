// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/shirou/gopsutil/host"

	commoncrash "chromiumos/tast/common/crash"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/platform/crash"
	"chromiumos/tast/local/chrome"
	localcrash "chromiumos/tast/local/crash"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

const (
	leaveCorePath            = "/root/.leave_core"
	crashReporterEnabledPath = "/var/lib/crash_reporter/crash-handling-enabled"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: UserCrash,
		Desc: "Verifies crash reporting for user processes",
		Contacts: []string{
			"domlaskowski@chromium.org", // Original autotest author
			"yamaguchi@chromium.org",    // Tast port author
			"cros-monitoring-forensics@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "metrics_consent"},
	})
}

// testReporterStartup tests that the core_pattern is set up by crash reporter.
func testReporterStartup(ctx context.Context, cr *chrome.Chrome, s *testing.State) {
	// Turn off crash filtering so we see the original setting.
	if err := localcrash.DisableCrashFiltering(); err != nil {
		s.Error("Failed to turn off crash filtering: ", err)
		return
	}
	out, err := ioutil.ReadFile(commoncrash.CorePattern)
	if err != nil {
		s.Error("Failed to read core pattern file: ", commoncrash.CorePattern)
		return
	}
	trimmed := strings.TrimSuffix(string(out), "\n")
	if expected := commoncrash.ExpectedCorePattern(); trimmed != expected {
		s.Errorf("Unexpected core_pattern: got %s, want %s", trimmed, expected)
	}

	// Check that we wrote out the file indicating that crash_reporter is
	// enabled AFTER the system was booted. This replaces the old technique
	// of looking for the log message which was flakey when the logs got
	// flooded.
	// NOTE: This technique doesn't need to be highly accurate, we are only
	// verifying that the flag was written after boot and there are multiple
	// seconds between those steps, and a file from a prior boot will almost
	// always have been written out much further back in time than our
	// current boot time.
	f, err := os.Stat(crashReporterEnabledPath)
	if err != nil || !f.Mode().IsRegular() {
		s.Error("Crash reporter enabled file flag is not present at ", crashReporterEnabledPath)
		return
	}
	flagTime := time.Since(f.ModTime())
	uptimeSeconds, err := host.Uptime()
	if err != nil {
		s.Error("Failed to get uptime: ", err)
		return
	}
	if flagTime > time.Duration(uptimeSeconds)*time.Second {
		s.Error("User space crash handling was not started during last boot")
	}
}

func unstashLeaveCore(ctx context.Context) {
	if err := testexec.CommandContext(ctx, "umount", "/root").Run(testexec.DumpLogOnError); err != nil {
		testing.ContextLog(ctx, "Failed to unmount: ", err)
	}
}

// stashLeaveCore prepares /root directory with .leave_core file eliminated.
// The first return value indicates whether the stashing happened, regardless
// if there was error after that. When it's true, the caller should call
// unstashLeaveCore() to resume the original /root directory.
func stashLeaveCore(ctx context.Context, cr *chrome.Chrome, s *testing.State) (retErr error) {
	fullCtx := ctx
	ctx, cancel := ctxutil.Shorten(fullCtx, 10*time.Second)
	defer cancel()
	// Avoid remounting / rw by instead creating a tmpfs in /root and
	// populating it with everything but the .leave_core file.
	for _, args := range [][]string{
		{"tar", "-cz", "-C", "/root", "-f", "/tmp/root.tgz", "."},
		{"mount", "-t", "tmpfs", "tmpfs", "/root"},
	} {
		if err := testexec.CommandContext(ctx, args[0], args[1:]...).Run(); err != nil {
			return errors.Wrapf(err, "%s failed", shutil.EscapeSlice(args))
		}
	}
	defer func() {
		if retErr != nil {
			unstashLeaveCore(fullCtx)
		}
	}()
	args := []string{"tar", "-xz", "-C", "/root", "-f", "/tmp/root.tgz", "."}
	if err := testexec.CommandContext(ctx, args[0], args[1:]...).Run(); err != nil {
		return errors.Wrapf(err, "%s failed", shutil.EscapeSlice(args))
	}
	if err := os.Remove("/tmp/root.tgz"); err != nil {
		return err
	}
	// /root/.leave_core always exists in a test image.
	if err := os.Remove(leaveCorePath); err != nil {
		return err
	}
	return nil
}

// testCoreFileRemovedInProduction tests core files do not stick around for production builds.
func testCoreFileRemovedInProduction(ctx context.Context, cr *chrome.Chrome, s *testing.State) {
	crash.ReporterVerboseLevel(2)
	defer crash.ReporterVerboseLevel(0)
	fullCtx := ctx
	ctx, cancel := ctxutil.Shorten(fullCtx, 10*time.Second)
	defer cancel()
	if err := stashLeaveCore(ctx, cr, s); err != nil {
		s.Fatal("Failed to stash .leave_core: ", err)
	}
	defer unstashLeaveCore(fullCtx)

	reader, err := syslog.NewReader(syslog.Program("crash_reporter"))
	if err != nil {
		s.Fatal("Failed to create log reader: ", err)
	}
	defer reader.Close()

	opts := crash.DefaultCrasherOptions()
	opts.Username = "root"
	if result, err := crash.RunCrasherProcess(ctx, cr, opts); err != nil {
		s.Fatal("Failed to run crasher process: ", err)
	} else if !result.Crashed {
		s.Fatal("Crasher did not crash")
	}
	crashDir, err := localcrash.GetCrashDir("root")
	if err != nil {
		s.Fatal("Failed opening root crash dir: ", err)
	}
	files, err := ioutil.ReadDir(crashDir)
	if err != nil {
		s.Fatal("Failed to read crash dir: ", err)
	}
	var crashContents []string
	for _, f := range files {
		crashContents = append(crashContents, f.Name())
	}
	testing.ContextLog(ctx, "Contents of crash directory: ", crashContents)
	const leavingCore = "Leaving core file at"
	for {
		e, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			s.Fatal("Failed to read syslog: ", err)
		}
		if strings.Contains(e.Content, leavingCore) {
			s.Errorf("Unexpected log message %q", leavingCore)
		}
	}

	dmpFiles := 0
	for _, n := range crashContents {
		if strings.HasSuffix(n, ".dmp") {
			dmpFiles++
		} else if strings.HasSuffix(n, ".core") {
			s.Error("Unexpected core file found: ", n)
		}
	}

	if dmpFiles != 1 {
		s.Errorf("Got %d dmp files, want 1", dmpFiles)
	}
}

// testReporterShutdown tests the crash_reporter shutdown code works.
func testReporterShutdown(ctx context.Context, cr *chrome.Chrome, s *testing.State) {
	cmd := testexec.CommandContext(ctx, commoncrash.CrashReporterPath, "--clean_shutdown")
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Error("Failed to clean shutdown crash reporter: ", err)
	}
	b, err := ioutil.ReadFile(commoncrash.CorePattern)
	if err != nil {
		s.Error("Failed to read core pattern file")
	}
	output := strings.TrimSpace(string(b))
	if output != "core" {
		s.Errorf("Core pattern was %q; want \"core\"", output)
	}
}

// testNoCrash tests that crasher can exit normally.
func testNoCrash(ctx context.Context, cr *chrome.Chrome, s *testing.State) {
	opts := crash.DefaultCrasherOptions()
	opts.Username = "root"
	opts.CauseCrash = false
	result, err := crash.RunCrasherProcessAndAnalyze(ctx, cr, opts)
	if err != nil {
		s.Error("testNoCrash failed: ", err)
		return
	}
	if result.Crashed || result.CrashReporterCaught || result.ReturnCode != 0 {
		s.Error("testNoCrash failed: not expecting crash")
	}
}

// testChronosCrasher tests that crasher exits by SIGSEGV with user "chronos".
func testChronosCrasher(ctx context.Context, cr *chrome.Chrome, s *testing.State) {
	opts := crash.DefaultCrasherOptions()
	opts.Username = "chronos"
	if err := crash.CheckCrashingProcess(ctx, cr, opts); err != nil {
		s.Error("testChronosCrasher failed: ", err)
	}
}

// testChronosCrasherNoConsent tests that no files are stored without consent, with user "chronos".
func testChronosCrasherNoConsent(ctx context.Context, cr *chrome.Chrome, s *testing.State) {
	if err := localcrash.SetConsent(ctx, cr, false); err != nil {
		s.Fatal("testChronosCrasherNoConsent failed: ", err)
	}
	opts := crash.DefaultCrasherOptions()
	opts.Consent = false
	opts.Username = "chronos"
	if err := crash.CheckCrashingProcess(ctx, cr, opts); err != nil {
		s.Error("testChronosCrasherNoConsent failed: ", err)
	}
}

// testRootCrasher tests that crasher exits by SIGSEGV with the root user.
func testRootCrasher(ctx context.Context, cr *chrome.Chrome, s *testing.State) {
	opts := crash.DefaultCrasherOptions()
	opts.Username = "root"
	if err := crash.CheckCrashingProcess(ctx, cr, opts); err != nil {
		s.Error("testRootCrasher failed: ", err)
	}
}

// testRootCrasherNoConsent tests that no files are stored without consent, with the root user.
func testRootCrasherNoConsent(ctx context.Context, cr *chrome.Chrome, s *testing.State) {
	if err := localcrash.SetConsent(ctx, cr, false); err != nil {
		s.Fatal("testRootCrasherNoConsent failed: ", err)
	}
	opts := crash.DefaultCrasherOptions()
	opts.Consent = false
	opts.Username = "root"
	if err := crash.CheckCrashingProcess(ctx, cr, opts); err != nil {
		s.Error("testRootCrasherNoConsent failed: ", err)
	}
}

// checkFilterCrasher runs crasher and verifies that crash_reporter receives or ignores the crash.
func checkFilterCrasher(ctx context.Context, shouldReceive bool) error {
	reader, err := syslog.NewReader()
	if err != nil {
		return err
	}
	defer reader.Close()
	cmd := testexec.CommandContext(ctx, crash.CrasherPath)
	if err := cmd.Run(testexec.DumpLogOnError); err == nil {
		return errors.Wrap(err, "crasher did not crash")
	} else if _, ok := err.(*exec.ExitError); !ok {
		return errors.Wrap(err, "failed to run crasher")
	}

	crasherBasename := filepath.Base(crash.CrasherPath)
	var expected string

	// Verify if crash_reporter received or not using log messages by that program.
	// These must be kept in sync with those in UserCollectorBase::HandleCrash in
	// src/platform2/crash-reporter/user_collector_base.cc.
	if shouldReceive {
		expected = "Received crash notification for " + crasherBasename
	} else {
		expected = "Ignoring crash from " + crasherBasename
	}

	if _, err := reader.Wait(ctx, 10*time.Second, func(e *syslog.Entry) bool {
		return strings.Contains(e.Content, expected)
	}); err != nil {
		return errors.Wrapf(err, "timeout waiting for %s in syslog", expected)
	}

	// crash_reporter may write log multiple times with different tags for certain message,
	// if it runs multiple "collectors" in it. (Currently it has user and ARC collectors.)
	// "Ignoring" message doesn't have a tag to simply identify which collector wrote it.
	// Wait until those messages are flushed. Otherwise next test will capture them wrongly.
	const successLog = "CheckFilterCrasher successfully verified."
	testing.ContextLog(ctx, successLog)
	if _, err := reader.Wait(ctx, 10*time.Second, func(e *syslog.Entry) bool {
		return strings.Contains(e.Content, successLog)
	}); err != nil {
		return errors.Wrapf(err, "timeout waiting for log flushed: want %q", successLog)
	}

	return nil
}

// testCrashFiltering tests that crash filtering (a feature needed for testing) works.
func testCrashFiltering(ctx context.Context, cr *chrome.Chrome, s *testing.State) {
	localcrash.EnableCrashFiltering("none")
	if err := checkFilterCrasher(ctx, false); err != nil {
		s.Error("testCrashFiltering failed for filter=\"none\": ", err)
	}

	localcrash.EnableCrashFiltering("sleep")
	if err := checkFilterCrasher(ctx, false); err != nil {
		s.Error("testCrashFiltering failed for filter=\"sleep\": ", err)
	}

	localcrash.DisableCrashFiltering()
	if err := checkFilterCrasher(ctx, true); err != nil {
		s.Error("testCrashFiltering failed for no-filter: ", err)
	}

	if err := localcrash.CleanCrashSpoolDirs(ctx); err != nil {
		s.Error("testCrashFiltering failed to cleanup: ", err)
	}
}

// checkCollectionFailure is a helper function for testing with crash log collection failures.
func checkCollectionFailure(ctx context.Context, cr *chrome.Chrome, testOption, failureString string) error {
	// Add parameter to core_pattern.
	out, err := ioutil.ReadFile(commoncrash.CorePattern)
	if err != nil {
		return errors.Wrapf(err, "failed to read core pattern file: %s", commoncrash.CorePattern)
	}
	oldCorePattern := strings.TrimSpace(string(out))
	if err := ioutil.WriteFile(commoncrash.CorePattern, []byte(oldCorePattern+" "+testOption), 0644); err != nil {
		return errors.Wrapf(err, "failed to add core pattern: %s", testOption)
	}
	defer func() {
		if err := ioutil.WriteFile(commoncrash.CorePattern, []byte(oldCorePattern), 0644); err != nil {
			testing.ContextLog(ctx, "Failed to restore core pattern file: ", err)
		}
	}()
	reader, err := syslog.NewReader()
	if err != nil {
		return errors.Wrap(err, "failed to create log reader")
	}
	defer reader.Close()
	opts := crash.DefaultCrasherOptions()
	opts.Username = "root"
	result, err := crash.RunCrasherProcessAndAnalyze(ctx, cr, opts)
	if err != nil {
		return errors.Wrap(err, "failed to call crasher")
	}
	if !result.Crashed {
		return errors.Errorf("crasher returned %d instead of crashing", result.ReturnCode)
	}
	if !result.CrashReporterCaught {
		return errors.New("logs do not contain crash_reporter message")
	}

	// RunCrasherProcessAndAnalyze already waits log output. Therefore the expected log should already be written.
	foundFailure := false
	for {
		entry, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return errors.Wrap(err, "failed to read log")
		}
		if strings.Contains(entry.Content, failureString) {
			foundFailure = true
			break
		}
	}
	if !foundFailure {
		return errors.Errorf("did not find fail string in the log: %s", failureString)
	}
	if result.Minidump != "" {
		return errors.New("failed collection resulted in minidump")
	}
	if result.Log == "" {
		return errors.New("failed collection had no log")
	}
	out, err = ioutil.ReadFile(result.Log)
	if err != nil {
		return err
	}
	logContents := string(out)
	if !strings.Contains(logContents, failureString) {
		return errors.Errorf("did not find %q in the result log %s", failureString, result.Log)
	}

	// Verify we are generating appropriate diagnostic output.
	if !strings.Contains(logContents, "===ps output===") || !strings.Contains(logContents, "===meminfo===") {
		return errors.Errorf("expected full logs in the result log %s", result.Log)
	}

	// TODO(crbug.com/970930): Check generated report sent.
	// The function is to be introduced by crrev.com/c/1906405.
	// const collectionErrorSignature = "crash_reporter-user-collection"
	// crash.CheckGeneratedReportSending(result.Meta, result.Log, result.Basename, "log", collectionErrorSignature)

	return nil
}

func testCore2mdFailure(ctx context.Context, cr *chrome.Chrome, s *testing.State) {
	const core2mdPath = "/usr/bin/core2md"
	if err := checkCollectionFailure(ctx, cr, "--core2md_failure", "Problem during "+core2mdPath+" [result=1]"); err != nil {
		s.Error("testCore2mdFailure failed: ", err)
	}
}

func testInternalDirectoryFailure(ctx context.Context, cr *chrome.Chrome, s *testing.State) {
	if err := checkCollectionFailure(ctx, cr, "--directory_failure", "Purposefully failing to create"); err != nil {
		s.Error("testInternalDirectoryFailure failed: ", err)
	}
}

func testCrashLogsCreation(ctx context.Context, cr *chrome.Chrome, s *testing.State) {
	// Copy and rename crasher to trigger crash_reporter_logs.conf rule.
	opts := crash.DefaultCrasherOptions()
	opts.Username = "root"
	opts.CrasherPath = filepath.Join(filepath.Dir(crash.CrasherPath), "crash_log_test")
	result, err := crash.RunCrasherProcessAndAnalyze(ctx, cr, opts)
	if err != nil {
		s.Fatal("Failed to run crasher: ", err)
	}
	if !result.Crashed {
		s.Errorf("Crasher returned %d instead of crashing", result.ReturnCode)
	}
	if !result.CrashReporterCaught {
		s.Error("Logs do not contain crash_reporter message")
	}
	b, err := ioutil.ReadFile(result.Log)
	if err != nil {
		s.Error("Failed to read result log: ", err)
	}
	if contents := string(b); contents != "hello world\n" {
		s.Error("Crash log contents unexpected: ", contents)
	}
	b, err = ioutil.ReadFile(result.Meta)
	if err != nil {
		s.Error("Failed to read result meta: ", err)
	}
	if !strings.Contains(string(b), "log="+filepath.Base(result.Log)) {
		s.Error("Meta file does not reference log")
	}
}

func testCrashLogInfiniteRecursion(ctx context.Context, cr *chrome.Chrome, s *testing.State) {
	// Copy and rename crasher to trigger crash_reporter_logs.conf rule.
	bindir := filepath.Dir(crash.CrasherPath)
	recursionTriggeringCrasher := filepath.Join(bindir, "crash_log_recursion_tast_test")

	// The configuration file hardcodes this path, so make sure it's still the same.
	// See /src/platform2/crash-reporter/crash_reporter_logs.conf
	const RecursionTestPath = "/usr/local/libexec/tast/helpers/local/cros/crash_log_recursion_tast_test"
	if recursionTriggeringCrasher != RecursionTestPath {
		s.Fatalf("Path to recursion test changed; want %s, got %s", RecursionTestPath, recursionTriggeringCrasher)
	}

	// Simply completing this command means that we avoided infinite recursion.
	opts := crash.DefaultCrasherOptions()
	opts.Username = "root"
	opts.CrasherPath = recursionTriggeringCrasher
	result, err := crash.RunCrasherProcess(ctx, cr, opts)
	if err != nil {
		s.Fatal("Failed to run crasher process: ", err)
	}
	if !result.Crashed {
		s.Errorf("Crasher returned %d instead of crashing", result.ReturnCode)
	}
	if !result.CrashReporterCaught {
		s.Error("Logs do not contain crash_reporter message")
	}
}

// testMaxEnqueuedCrash tests that the maximum crash directory size is enforced.
func testMaxEnqueuedCrash(ctx context.Context, cr *chrome.Chrome, s *testing.State) {
	const (
		maxCrashDirectorySize = 32
		username              = "root"
	)
	reader, err := syslog.NewReader()
	defer reader.Close()
	if err != nil {
		s.Fatal("Failed to create watcher: ", err)
	}
	crashDir, err := localcrash.GetCrashDir(username)
	if err != nil {
		s.Fatal("Failed before queueing: ", err)
	}
	fullMessage := fmt.Sprintf("Crash directory %s already full with %d pending reports",
		crashDir, maxCrashDirectorySize)
	opts := crash.DefaultCrasherOptions()
	opts.Username = username

	// Fill up the queue.
	for i := 0; i < maxCrashDirectorySize; i++ {
		result, err := crash.RunCrasherProcess(ctx, cr, opts)
		if err != nil {
			s.Fatal("Failure while setting up queue: ", err)
		}
		if !result.Crashed {
			s.Fatal("Failure while setting up queue: ", result.ReturnCode)
		}
		if !result.CrashReporterCaught {
			s.Fatal("Crash reporter did not handle while setting up queue")
		}
		for {
			e, err := reader.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				s.Fatal("Failed to read syslog: ", err)
			}
			if strings.Contains(e.Content, fullMessage) {
				s.Fatal("Unexpected full message: ", e.Content)
			}
		}
	}

	files, err := ioutil.ReadDir(crashDir)
	if err != nil {
		s.Fatal("Failed to get crash dir size: ", crashDir)
	}
	crashDirSize := len(files)
	testing.ContextLogf(ctx, "Crash directory had %d entries", crashDirSize)

	// Crash a bunch more times, but make sure no new reports are enqueued.
	for i := 0; i < 10; i++ {
		result, err := crash.RunCrasherProcess(ctx, cr, opts)
		if err != nil {
			s.Fatal("Failure while running crasher after enqueued: ", err)
		}
		if !result.Crashed {
			s.Fatal("Failure after setting up queue: ", result.ReturnCode)
		}
		if !result.CrashReporterCaught {
			s.Fatal("Crash reporter did not catch crash")
		}
		if _, err := reader.Wait(ctx, 20*time.Second, func(e *syslog.Entry) bool { return strings.Contains(e.Content, fullMessage) }); err != nil {
			s.Error("Expected full message: ", fullMessage)
		}
		files, err = ioutil.ReadDir(crashDir)
		if err != nil {
			s.Fatalf("Failed to get crash dir size of %s: %v", crashDir, err)
		}
		if crashDirSize != len(files) {
			s.Errorf("Expected no new files (now %d, were %d)", len(files), crashDirSize)
		}
	}
}

func UserCrash(ctx context.Context, s *testing.State) {
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to restart UI job")
	}

	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	// Run all tests.
	crash.RunCrashTests(ctx, cr, s, []func(context.Context, *chrome.Chrome, *testing.State){
		testReporterStartup,
		testCoreFileRemovedInProduction,
		testReporterShutdown,
		testNoCrash,
		testChronosCrasher,
		testChronosCrasherNoConsent,
		testRootCrasher,
		testRootCrasherNoConsent,
		testCrashFiltering,
		testMaxEnqueuedCrash,
		testCore2mdFailure,
		testInternalDirectoryFailure,
		testCrashLogsCreation,
		testCrashLogInfiniteRecursion,
	}, true)
}
