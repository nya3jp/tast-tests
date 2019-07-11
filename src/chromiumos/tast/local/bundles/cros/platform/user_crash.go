// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"

	"chromiumos/tast/local/bundles/cros/platform/crash"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const (
	leaveCorePath = "/root/.leave_core"
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

func umountRoot(ctx context.Context, s *testing.State) {
	if err := testexec.CommandContext(ctx, "umount", "/root").Run(testexec.DumpLogOnError); err != nil {
		s.Error("Failed to unmount: ", err)
	}
}

// testReporterStartup tests that the core_pattern is set up by crash reporter.
func testReporterStartup(ctx context.Context, s *testing.State) {
	// Turn off crash filtering so we see the original setting.
	if err := crash.DisableCrashFiltering(); err != nil {
		s.Fatal("Failed to turn off crash filtering: ", err)
	}
	out, err := ioutil.ReadFile(crash.CorePattern)
	if err != nil {
		s.Fatal("Failed to read core pattern file: ", crash.CorePattern)
	}
	trimmed := strings.TrimSuffix(string(out), "\n")
	expectedCorePattern := fmt.Sprintf("|%s --user=%%P:%%s:%%u:%%g:%%e", crash.CrashReporterPath)
	if trimmed != expectedCorePattern {
		s.Errorf("core pattern should have been %s, not %s", expectedCorePattern, trimmed)
	}

	// Find log line of crash_reporter during the last boot.
	out, err = testexec.CommandContext(ctx, "journalctl", "-b", "0", "-q", "-t", "crash_reporter",
		"-g", "Enabling user crash handling").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to execute and get output result of journalctl: ", err)
	}
	if len(out) == 0 {
		s.Error("user space crash handling was not started during last boot")
	}
}

func UserCrash(ctx context.Context, s *testing.State) {
	if err := crash.Setup(ctx); err != nil {
		s.Fatal("Failed to set up crash test: ", err)
	}
	// TODO(crrev.com/970930): Restart UI if exist, when adding tests that
	// require it.
	testReporterStartup(ctx, s)
	if err := crash.TearDown(ctx); err != nil {
		s.Fatal("Failed to tear down crash test: ", err)
	}
}
