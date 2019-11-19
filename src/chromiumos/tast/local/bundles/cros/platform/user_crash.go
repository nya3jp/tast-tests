// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/shirou/gopsutil/host"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/platform/crash"
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
	})
}

// testReporterStartup tests that the core_pattern is set up by crash reporter.
func testReporterStartup(ctx context.Context, s *testing.State) {
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

// testNoCrash tests that crasher can exit normally.
func testNoCrash(ctx context.Context, s *testing.State) {
	opts := crash.DefaultCrasherOptions()
	opts.Username = "root"
	opts.CauseCrash = false
	result, err := crash.RunCrasherProcessAndAnalyze(ctx, opts)
	if err != nil {
		s.Error("testNoCrash failed: ", err)
		return
	}
	if result.Crashed || result.CrashReporterCaught || result.ReturnCode != 0 {
		s.Error("testNoCrash failed: not expecting crash")
	}
}

// testChronosCrasher tests that crasher exits by SIGSEGV with user "chronos".
func testChronosCrasher(ctx context.Context, s *testing.State) {
	opts := crash.DefaultCrasherOptions()
	opts.Username = "chronos"
	if err := crash.CheckCrashingProcess(ctx, opts); err != nil {
		s.Error("testChronosCrasher failed: ", err)
	}
}

// testRootCrasher tests that crasher exits by SIGSEGV with the root user.
func testRootCrasher(ctx context.Context, s *testing.State) {
	opts := crash.DefaultCrasherOptions()
	opts.Username = "root"
	if err := crash.CheckCrashingProcess(ctx, opts); err != nil {
		s.Error("testRootCrasher failed: ", err)
	}
}

func checkFilterCrasher(ctx context.Context, shouldReceive bool) error {
	watcher, err := syslog.NewWatcher("/var/log/messages")
	if err != nil {
		return err
	}
	cmd := testexec.CommandContext(ctx, crash.CrasherPath)
	if err := cmd.Run(testexec.DumpLogOnError); err == nil {
		return errors.Wrap(err, "crasher did not crash")
	} else if _, ok := err.(*exec.ExitError); !ok {
		return errors.Wrap(err, "failed to run crasher")
	}

	crasherBasename := filepath.Base(crash.CrasherPath)
	var expected string
	if shouldReceive {
		expected = "Received crash notification for " + crasherBasename
	} else {
		expected = "Ignoring crash from " + crasherBasename
	}

	c, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := watcher.WaitForMessage(c, expected); err != nil {
		return errors.Wrapf(err, "timeout waiting for %s in syslog", expected)
	}

	// crash_reporter may write log multiple times with different tags for certain message,
	// if it runs multiple "collectors" in it. (Currently it has user and arc collectors.)
	// "Ignoring" message doesn't have a tag to simply identify which collector wrote it.
	// Wait until those messages are flushed. Otherwise next test will capture them wrongly.
	const successLog = "CheckFilterCrasher successfully verified."
	testing.ContextLog(ctx, successLog)
	c, cancel = context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := watcher.WaitForMessage(c, successLog); err != nil {
		return errors.Wrapf(err, "timeout waiting for log flushed: want %q", successLog)
	}

	return nil
}

// testCrashFiltering tests that crash filtering (a feature needed for testing) works.
func testCrashFiltering(ctx context.Context, s *testing.State) {
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

func UserCrash(ctx context.Context, s *testing.State) {
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to restart UI job")
	}

	// TODO(yamaguchi): Uncomment this when the crash.go supoprts push/popping consent files.
	// Run the test once without re-initializing to catch problems with the default crash reporting setup
	// crash.RunCrashTests(ctx, s, []func(context.Context, *testing.State){testReporterStartup}, false)

	// Run all tests.
	crash.RunCrashTests(ctx, s, []func(context.Context, *testing.State){
		testReporterStartup,
		testNoCrash,
		testChronosCrasher,
		testRootCrasher,
		testCrashFiltering,
	}, true)
}
