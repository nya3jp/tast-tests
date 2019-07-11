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

	"chromiumos/tast/local/bundles/cros/platform/crash"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
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

func umountRoot(c context.Context, s *testing.State) {
	args := []string{"umount", "/root"}
	if err := testexec.CommandContext(c, args[0], args[1:]...).Run(); err != nil {
		s.Errorf("%s failed: %v", shutil.EscapeSlice(args), err)
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
	cmd := testexec.CommandContext(ctx, "journalctl", "-b", "0", "-q",
		"-t", "crash_reporter", "-g", "Enabling user crash handling")
	out, err = cmd.Output()
	if err != nil {
		s.Fatal("Failed to execute and get output result of journalctl: ", err)
	}
	if len(out) == 0 {
		s.Error("user space crash handling was not started during last boot")
	}
}

// test that core files do not stick around for production builds.
func testCoreFileRemovedInProduction(ctx context.Context, s *testing.State) {
	// Avoid remounting / rw by instead creating a tmpfs in /root and
	// populating it with everything but the
	for _, args := range [][]string{
		{"tar", "-cvz", "-C", "/root", "-f", "/tmp/root.tgz", "."},
		{"mount", "-t", "tmpfs", "tmpfs", "/root"},
	} {
		if err := testexec.CommandContext(ctx, args[0], args[1:]...).Run(); err != nil {
			s.Fatalf("%s failed: %v", shutil.EscapeSlice(args), err)
		}
	}
	defer umountRoot(ctx, s)
	args := []string{"tar", "-xvz", "-C", "/root", "-f", "/tmp/root.tgz", "."}
	if err := testexec.CommandContext(ctx, args[0], args[1:]...).Run(); err != nil {
		s.Fatalf("%s failed: %v", shutil.EscapeSlice(args), err)
	}
	if err := os.Remove(leaveCorePath); err != nil {
		s.Fatal("Failed to remove .leave_core: ", err)
	}
	if _, err := os.Stat(leaveCorePath); err == nil {
		s.Fatal(".leave_core file did not disappear")
	}
}

func UserCrash(ctx context.Context, s *testing.State) {
	testReporterStartup(ctx, s)
	testCoreFileRemovedInProduction(ctx, s)
}
