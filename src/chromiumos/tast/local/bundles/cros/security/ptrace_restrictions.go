// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PtraceRestrictions,
		Desc: "Checks that the kernel restricts ptrace between processes",
		Attr: []string{"informational"},
	})
}

func PtraceRestrictions(ctx context.Context, s *testing.State) {
	const sysctl = "/proc/sys/kernel/yama/ptrace_scope"
	b, err := ioutil.ReadFile(sysctl)
	if err != nil {
		s.Fatalf("Failed to read %v: %v", sysctl, err)
	}
	if str := strings.TrimSpace(string(b)); str != "1" {
		s.Fatalf("%v contains %q; want \"1\"", sysctl, str)
	}

	const (
		dir  = "/usr/local/libexec/security_tests/security.PtraceRestrictions"
		user = "chronos" // unprivileged user
	)

	runCmd := func(exec string, args ...string) {
		if ctx.Err() != nil {
			return
		}
		cmd := testexec.CommandContext(ctx, exec, args...)
		cmd.Dir = dir
		cmdStr := testexec.ShellEscapeArray(cmd.Args)
		s.Log("Running ", cmdStr)
		if err := cmd.Run(); err != nil {
			s.Errorf("%v failed: %v", cmdStr, err)
			cmd.DumpLog(ctx)
		}
	}

	// Verify that ptrace is only allowed on children or declared processes.
	runCmd("sudo", "-u", user, "bash", "-x", filepath.Join(dir, "ptrace-restrictions.sh"))

	// Verify that ptrace can be made to work across PID namespaces.
	runCmd("bash", "-x", filepath.Join(dir, "root-ptrace-restrictions.sh"), user)

	type behavior int
	const (
		tracerForksTracee                   behavior = 0
		traceeCallsPrctlFromMainProcess              = 1
		traceeCallsPrctlFromNonLeaderThread          = 2
	)

	type source int
	const (
		ptraceFromNonLeaderThread source = 0
		ptraceFromMainProcess            = 1
	)

	runThreadTest := func(bh behavior, src source) {
		runCmd("sudo", "-u", user, filepath.Join(dir, "thread-prctl"), strconv.Itoa(int(bh)), strconv.Itoa(int(src)))
	}

	// Verify that ptrace of child is permitted from parent process and thread.
	runThreadTest(tracerForksTracee, ptraceFromMainProcess)
	runThreadTest(tracerForksTracee, ptraceFromNonLeaderThread)

	// Verify that prctl(PR_SET_PTRACER, ...) is permitted from main process and thread.
	runThreadTest(traceeCallsPrctlFromMainProcess, ptraceFromMainProcess)
	runThreadTest(traceeCallsPrctlFromNonLeaderThread, ptraceFromMainProcess)

	// Verify that ptrace is permitted from thread on process that used PR_SET_PTRACER.
	runThreadTest(traceeCallsPrctlFromMainProcess, ptraceFromNonLeaderThread)
	runThreadTest(traceeCallsPrctlFromNonLeaderThread, ptraceFromNonLeaderThread)
}
