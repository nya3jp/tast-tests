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

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/platform/crash"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
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
		},
		Attr: []string{"group:mainline", "informational"},
		// chrome_internal because only official builds are even considered to have
		// metrics consent; see ChromeCrashReporterClient::GetCollectStatsConsent()
		SoftwareDeps: []string{"chrome", "chrome_internal"},
	})
}

// testReporterStartup tests that the core_pattern is set up by crash reporter.
func testReporterStartup(ctx context.Context, cr *chrome.Chrome, s *testing.State) {
	// Turn off crash filtering so we see the original setting.
	if err := crash.DisableCrashFiltering(); err != nil {
		s.Error("Failed to turn off crash filtering: ", err)
		return
	}
	out, err := ioutil.ReadFile(crash.CorePattern)
	if err != nil {
		s.Error("Failed to read core pattern file: ", crash.CorePattern)
		return
	}
	trimmed := strings.TrimSuffix(string(out), "\n")
	expectedCorePattern := fmt.Sprintf("|%s --user=%%P:%%s:%%u:%%g:%%e", crash.CrashReporterPath)
	if trimmed != expectedCorePattern {
		s.Errorf("Unexpected core_pattern: got %s, want %s", trimmed, expectedCorePattern)
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

// testReporterShutdown tests the crash_reporter shutdown code works.
func testReporterShutdown(ctx context.Context, cr *chrome.Chrome, s *testing.State) {
	cmd := testexec.CommandContext(ctx, crash.CrashReporterPath, "--clean_shutdown")
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Error("Failed to clean shutdown crash reporter: ", err)
	}
	b, err := ioutil.ReadFile(crash.CorePattern)
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

// testChronosCrasherNoConsent tests that crasher exits by SIGSEGV with user "chronos".
func testChronosCrasherNoConsent(ctx context.Context, cr *chrome.Chrome, s *testing.State) {
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

// testRootCrasherNoConsent tests that crasher exits by SIGSEGV with the root user.
func testRootCrasherNoConsent(ctx context.Context, cr *chrome.Chrome, s *testing.State) {
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
	crash.EnableCrashFiltering("none")
	if err := checkFilterCrasher(ctx, false); err != nil {
		s.Error("testCrashFiltering failed for filter=\"none\": ", err)
	}

	crash.EnableCrashFiltering("sleep")
	if err := checkFilterCrasher(ctx, false); err != nil {
		s.Error("testCrashFiltering failed for filter=\"sleep\": ", err)
	}

	crash.DisableCrashFiltering()
	if err := checkFilterCrasher(ctx, true); err != nil {
		s.Error("testCrashFiltering failed for no-filter: ", err)
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
	crashDir, err := crash.GetCrashDir(username)
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

	cr, err := chrome.New(ctx, chrome.KeepState())
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	// TODO(yamaguchi): Uncomment this when the crash.go supoprts push/popping consent files.
	// Run the test once without re-initializing to catch problems with the default crash reporting setup
	// crash.RunCrashTests(ctx, s, []func(context.Context, *testing.State){testReporterStartup}, false)

	// Run all tests.
	crash.RunCrashTests(ctx, cr, s, []func(context.Context, *chrome.Chrome, *testing.State){
		testReporterStartup,
		testReporterShutdown,
		testNoCrash,
		testChronosCrasher,
		testRootCrasher,
		testCrashFiltering,
		testMaxEnqueuedCrash,
		testCrashLogsCreation,
		testCrashLogInfiniteRecursion,
	}, true, true)
	crash.RunCrashTests(ctx, cr, s, []func(context.Context, *chrome.Chrome, *testing.State){
		testChronosCrasherNoConsent,
		testRootCrasherNoConsent,
	}, false, true)
}
