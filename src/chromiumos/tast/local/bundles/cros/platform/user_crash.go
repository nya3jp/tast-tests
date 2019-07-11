// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/shirou/gopsutil/host"

	"chromiumos/tast/local/bundles/cros/platform/crash"
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
		Attr: []string{"informational"},
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

func UserCrash(ctx context.Context, s *testing.State) {
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to restart UI job")
	}

	// TODO(yamaguchi): Uncomment this when the crash.go supoprts push/popping consent files.
	// Run the test once without re-initializing to catch problems with the default crash reporting setup
	// crash.RunCrashTests(ctx, s, []func(context.Context, *testing.State){testReporterStartup}, false)

	// Run all tests.
	crash.RunCrashTests(ctx, s, []func(context.Context, *testing.State){testReporterStartup}, true)
}
